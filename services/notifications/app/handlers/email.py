import logging
import json
from datetime import datetime, timezone
from app.models.event import OrderEvent, NotificationResult
from config.settings import settings

# Setup structured logger
logger = logging.getLogger(__name__)


class EmailHandler:
    """
    Handles sending email notifications.
    In local/dry_run mode: just logs the email.
    In production on EKS: sends via AWS SES.
    """

    def __init__(self):
        self.dry_run = settings.dry_run
        if self.dry_run:
            logger.info(json.dumps({
                "service": "notifications",
                "message": "EmailHandler running in DRY RUN mode - emails will be logged only"
            }))

    async def send_order_confirmation(self, event: OrderEvent) -> NotificationResult:
        """
        Sends an order confirmation email.
        Called when order.placed event is received from RabbitMQ.
        """
        try:
            if self.dry_run:
                # Local development — just log it
                # This still shows up in CloudWatch when deployed to EKS
                logger.info(json.dumps({
                    "service": "notifications",
                    "level": "info",
                    "message": "EMAIL SENT (dry run)",
                    "event_id": event.event_id,
                    "event_type": event.event_type,
                    "order_id": event.order_id,
                    "user_id": event.user_id,
                    "item_name": event.item_name,
                    "quantity": event.quantity,
                    "price": event.price,
                    "email_subject": f"Order Confirmation - Order #{event.order_id}",
                    "email_body": f"Your order for {event.item_name} x{event.quantity} has been placed successfully!",
                    "timestamp": datetime.now(timezone.utc).isoformat()
                }))
            else:
                # Production — send via AWS SES
                await self._send_via_ses(event)

            return NotificationResult(
                success=True,
                event_id=event.event_id,
                order_id=event.order_id,
                message="Email notification sent successfully",
                processed_at=datetime.now(timezone.utc)
            )

        except Exception as e:
            logger.error(json.dumps({
                "service": "notifications",
                "level": "error",
                "message": "Failed to send email",
                "event_id": event.event_id,
                "order_id": event.order_id,
                "error": str(e)
            }))
            return NotificationResult(
                success=False,
                event_id=event.event_id,
                order_id=event.order_id,
                message=f"Failed to send email: {str(e)}",
                processed_at=datetime.now(timezone.utc)
            )

    async def _send_via_ses(self, event: OrderEvent):
        """
        Sends email via AWS SES.
        Only called in production — not in local development.
        """
        import boto3
        ses = boto3.client("ses", region_name=settings.aws_region)

        ses.send_email(
            Source="noreply@shopsre.com",
            Destination={"ToAddresses": [f"user_{event.user_id}@example.com"]},
            Message={
                "Subject": {
                    "Data": f"Order Confirmation - Order #{event.order_id}"
                },
                "Body": {
                    "Text": {
                        "Data": (
                            f"Your order has been placed!\n\n"
                            f"Order ID: {event.order_id}\n"
                            f"Item: {event.item_name}\n"
                            f"Quantity: {event.quantity}\n"
                            f"Price: ${event.price}\n"
                        )
                    }
                }
            }
        )