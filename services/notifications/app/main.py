import asyncio
import json
import logging
import os
import sys
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from prometheus_client import make_asgi_app

from app.consumers.rabbitmq import RabbitMQConsumer
from config.settings import settings

# Setup structured JSON logging
# All logs go to CloudWatch when running on EKS
logging.basicConfig(
    stream=sys.stdout,
    level=logging.INFO,
    format="%(message)s"  # just the JSON message — no extra formatting
)
logger = logging.getLogger(__name__)

# Global consumer instance
consumer = RabbitMQConsumer()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    Runs on startup and shutdown.
    Starts the RabbitMQ consumer in background.
    """
    # Startup
    logger.info(json.dumps({
        "service": "notifications",
        "message": f"Starting Notifications Service on port {settings.port}",
        "env": settings.env,
        "dry_run": settings.dry_run
    }))

    # Start RabbitMQ consumer in background task
    # It runs independently of the HTTP server
    consumer_task = asyncio.create_task(start_consumer())

    yield  # App is running

    # Shutdown
    logger.info(json.dumps({
        "service": "notifications",
        "message": "Shutting down Notifications Service..."
    }))
    consumer_task.cancel()
    await consumer.close()


async def start_consumer():
    """Start RabbitMQ consumer with retry logic"""
    while True:
        try:
            await consumer.start_consuming()
        except Exception as e:
            logger.error(json.dumps({
                "service": "notifications",
                "level": "error",
                "message": f"Consumer error: {e} - retrying in 5 seconds"
            }))
            await asyncio.sleep(5)


# Create FastAPI app
app = FastAPI(
    title="Notifications Service",
    version="1.0.0",
    lifespan=lifespan
)

# Mount Prometheus metrics at /metrics
# Prometheus scrapes this every 15 seconds
metrics_app = make_asgi_app()
app.mount("/metrics", metrics_app)


@app.get("/healthz")
async def healthz():
    """
    Health check endpoint.
    Kubernetes calls this every 10 seconds.
    If this returns 200, pod stays alive.
    """
    return JSONResponse(content={
        "status": "healthy",
        "service": "notifications",
        "env": settings.env
    })


@app.get("/api/notifications/status")
async def status():
    """
    Returns current status of the notifications service.
    Button 3 on the frontend calls this to show service is alive.
    """
    return JSONResponse(content={
        "status": "running",
        "service": "notifications",
        "dry_run": settings.dry_run,
        "message": "Notifications service is consuming from RabbitMQ"
    })


if __name__ == "__main__":
    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=int(settings.port),
        log_level="info"
    )