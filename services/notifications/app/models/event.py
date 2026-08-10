from pydantic import BaseModel
from datetime import datetime
from typing import Optional


class OrderEvent(BaseModel):
    """
    This is the exact message structure that Orders Service
    publishes to RabbitMQ when an order is placed.
    Notifications Service consumes this message.
    """
    event_id: str        # unique ID — used for idempotency
    event_type: str      # "order.placed"
    order_id: int
    user_id: int
    item_name: str
    quantity: int
    price: float
    timestamp: datetime


class NotificationResult(BaseModel):
    """
    Result of processing a notification
    """
    success: bool
    event_id: str
    order_id: int
    message: str
    processed_at: Optional[datetime] = None