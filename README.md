# \# ShopSRE Platform

# 

# Production-grade microservices on EKS — built to mirror real enterprise

# SRE and Platform Engineering workflows.

# 

# \## What is this?

# 

# A working e-commerce platform with a React UI containing 5 buttons.

# Each button triggers a real microservice chain that you can observe,

# trace, and troubleshoot end to end.

# 

# \## Services

# 

# | Service | Language | Purpose |

# |---|---|---|

# | Frontend | React + nginx | UI with 5 buttons |

# | API Gateway | Go | Single entry point, routing, rate limiting |

# | Auth | Go | Register, login, JWT issuance |

# | Orders | Go | Create and read orders |

# | Notifications | Python | Consumes events, sends emails |

# | Worker | Go | Background jobs, OOM simulator |

# 

# \## Observability

# 

# \- Prometheus — metrics from every service

# \- Grafana — SLO dashboards and alerting

# \- Jaeger — distributed tracing across all services

# \- Loki — structured logs correlated with traces

# 

# \## Tech Stack

# 

# Kubernetes (EKS) · Terraform · Helm · GitHub Actions ·

# PostgreSQL (RDS) · Redis (ElastiCache) · RabbitMQ (Amazon MQ)

# 

# \## Run locally

# 

# docker compose up --build

# 

# Open http://localhost:3000

