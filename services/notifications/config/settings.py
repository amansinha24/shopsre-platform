import os


class Settings:
    # Server
    port: str = os.getenv("PORT", "8083")

    # RabbitMQ
    rabbitmq_url: str = os.getenv(
        "RABBITMQ_URL",
        "amqp://admin:changeme_local@localhost:5672/"
    )

    # Redis
    redis_host: str = os.getenv("REDIS_HOST", "localhost")
    redis_port: str = os.getenv("REDIS_PORT", "6379")
    redis_password: str = os.getenv("REDIS_PASSWORD", "")

    # AWS
    aws_region: str = os.getenv("AWS_REGION", "us-east-1")

    # Environment
    env: str = os.getenv("ENV", "local")

    # Dry run mode
    # In local development we just log emails instead of sending them
    dry_run: bool = os.getenv("DRY_RUN", "true").lower() == "true"


# Single instance used across the app
settings = Settings()