import asyncio
import json
import logging
from datetime import datetime, timezone

import aio_pika
import redis.asyncio as aioredis
from prometheus_client import Counter, Histogram

from app.handlers.email import EmailHandler
from app.models.event import OrderEvent
from config.settings import settings

logger = logging.getLogger(__name__)

# Prometheus metrics
events_consumed_total = Counter(
    "notifications_events_consumed_total",
    "Total number of RabbitMQ events consumed",
    ["event_type", "status"]
)

event_processing_duration = Histogram(
    "notifications_event_processing_duration_seconds",
    "Time to process each event",
    buckets=[0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0]
)

duplicate_events_total = Counter(
    "notifications_duplicate_events_total",
    "Total number of duplicate events skipped"
)


class RabbitMQConsumer:
    """
    Consumes order.placed events from RabbitMQ.
    Uses Redis to track processed event IDs for idempotency.
    """

    def __init__(self):
        self.email_handler = EmailHandler()
        self.redis_client = None
        self.connection = None
        self.channel = None

    async def connect(self):
        """Connect to RabbitMQ and Redis"""
        # Connect to RabbitMQ
        logger.info(json.dumps({
            "service": "notifications",
            "message": "Connecting to RabbitMQ..."
        }))

        self.connection = await aio_pika.connect_robust(
            settings.rabbitmq_url,
            # Reconnect automatically if connection drops
            reconnect_interval=5
        )

        self.channel = await self.connection.channel()

        # prefetch_count=1 means only process one message at a time
        # This prevents the pod from hoarding messages before dying
        await self.channel.set_qos(prefetch_count=1)

        logger.info(json.dumps({
            "service": "notifications",
            "message": "Connected to RabbitMQ successfully"
        }))

        # Connect to Redis for idempotency checks
        try:
            self.redis_client = aioredis.Redis(
                host=settings.redis_host,
                port=int(settings.redis_port),
                password=settings.redis_password or None,
                decode_responses=True
            )
            await self.redis_client.ping()
            logger.info(json.dumps({
                "service": "notifications",
                "message": "Connected to Redis successfully"
            }))
        except Exception as e:
            logger.warning(json.dumps({
                "service": "notifications",
                "message": f"Redis connection failed: {e} - idempotency disabled"
            }))
            self.redis_client = None

    async def start_consuming(self):
        """Start consuming messages from the notifications queue"""
        await self.connect()

        # Declare the queue — same as what Orders service declared
        queue = await self.channel.declare_queue(
            "notifications.orders",
            durable=True,
            arguments={
                "x-dead-letter-exchange": "orders.dlx",
                "x-message-ttl": 86400000
            }
        )

        logger.info(json.dumps({
            "service": "notifications",
            "message": "Started consuming from notifications.orders queue"
        }))

        # Start consuming — process_message is called for each message
        await queue.consume(self.process_message)

        # Keep running forever
        await asyncio.Future()

    async def process_message(self, message: aio_pika.IncomingMessage):
        """
        Process a single message from RabbitMQ.
        This is called automatically for each message.
        """
        import time
        start_time = time.time()

        async with message.process(requeue=True):
            try:
                # Parse the message body
                body = json.loads(message.body.decode())
                event = OrderEvent(**body)

                logger.info(json.dumps({
                    "service": "notifications",
                    "message": "Received event",
                    "event_id": event.event_id,
                    "event_type": event.event_type,
                    "order_id": event.order_id
                }))

                # Idempotency check
                # If we already processed this event, skip it
                if await self._is_duplicate(event.event_id):
                    logger.info(json.dumps({
                        "service": "notifications",
                        "message": "Duplicate event skipped",
                        "event_id": event.event_id
                    }))
                    duplicate_events_total.inc()
                    return

                # Process based on event type
                if event.event_type == "order.placed":
                    result = await self.email_handler.send_order_confirmation(event)

                    if result.success:
                        # Mark as processed in Redis
                        await self._mark_processed(event.event_id)
                        events_consumed_total.labels(
                            event_type=event.event_type,
                            status="success"
                        ).inc()
                    else:
                        events_consumed_total.labels(
                            event_type=event.event_type,
                            status="failure"
                        ).inc()
                        # Raise exception to trigger requeue
                        raise Exception(result.message)
                else:
                    logger.warning(json.dumps({
                        "service": "notifications",
                        "message": f"Unknown event type: {event.event_type}"
                    }))

                # Record processing duration
                duration = time.time() - start_time
                event_processing_duration.observe(duration)

            except Exception as e:
                duration = time.time() - start_time
                event_processing_duration.observe(duration)

                logger.error(json.dumps({
                    "service": "notifications",
                    "level": "error",
                    "message": "Failed to process message",
                    "error": str(e),
                    "duration_seconds": duration
                }))
                raise

    async def _is_duplicate(self, event_id: str) -> bool:
        """Check if event was already processed using Redis"""
        if not self.redis_client:
            return False
        key = f"processed:notification:{event_id}"
        result = await self.redis_client.exists(key)
        return result > 0

    async def _mark_processed(self, event_id: str):
        """Mark event as processed in Redis with 24 hour expiry"""
        if not self.redis_client:
            return
        key = f"processed:notification:{event_id}"
        # Expires after 24 hours
        await self.redis_client.setex(key, 86400, "1")

    async def close(self):
        """Clean up connections"""
        if self.channel:
            await self.channel.close()
        if self.connection:
            await self.connection.close()
        if self.redis_client:
            await self.redis_client.close()