# Real-Time Chat Application

## Backend Architecture Design Document

**System Design Exercise**

**Primary Language:** Golang  
**Architecture Type:** Microservices with Event-Driven Design

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [System Architecture Overview](#2-system-architecture-overview)
3. [Core Components](#3-core-components)
4. [Data Flow: Message Send/Receive](#4-data-flow-message-sendreceive)
5. [Technology Choices & Rationale](#5-technology-choices--rationale)
6. [Scalability Strategy](#6-scalability-strategy)
7. [Fault Tolerance & Reliability](#7-fault-tolerance--reliability)
8. [Load Balancing Strategy](#8-load-balancing-strategy)
9. [Message Consistency & Ordering](#9-message-consistency--ordering)
10. [Database Schema Design](#10-database-schema-design)
11. [API Design](#11-api-design)
12. [Monitoring & Observability](#12-monitoring--observability)
13. [Security Considerations](#13-security-considerations)
14. [Deployment Architecture](#14-deployment-architecture)
15. [Summary](#15-summary)

---

## 1. Executive Summary

This document presents a comprehensive backend architecture design for a real-time chat application similar to Slack or WhatsApp. The system is designed to handle millions of concurrent users with sub-100ms message delivery latency, 99.99% uptime, and seamless horizontal scalability.

### Key Design Goals

- **Real-time message delivery:** Sub-100ms end-to-end latency for message delivery using WebSocket connections with intelligent routing
- **Massive scale:** Support for 10M+ concurrent connections with linear horizontal scaling capability
- **High availability:** 99.99% uptime with automatic failover, no single point of failure, and graceful degradation
- **Message consistency:** Guaranteed message ordering within conversations with exactly-once delivery semantics
- **Extensibility:** Microservices architecture enabling independent scaling and feature development

---

## 2. System Architecture Overview

The architecture follows a microservices pattern with event-driven communication. Each service is independently deployable, scalable, and maintainable. The system uses a combination of synchronous (gRPC) and asynchronous (Kafka) communication patterns.

### High-Level Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLIENT LAYER                                   │
│    ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│    │  Web App │  │Mobile iOS│  │ Android  │  │ Desktop  │                  │
│    └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘                  │
│         └──────────────┼──────────────┼──────────────┘                      │
│                        ▼              ▼                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                             EDGE LAYER                                      │
│    ┌───────────────────────────────────────────────────────────────────┐   │
│    │              CDN / Global Load Balancer (CloudFlare)              │   │
│    └─────────────────────────────┬─────────────────────────────────────┘   │
│                                  ▼                                          │
│    ┌───────────────────────────────────────────────────────────────────┐   │
│    │         API Gateway (Rate Limiting, Auth, Request Routing)        │   │
│    └─────────────────────────────┬─────────────────────────────────────┘   │
├──────────────────────────────────┼──────────────────────────────────────────┤
│                            SERVICE LAYER                                    │
│         ┌────────────────────────┴─────────────────────────┐               │
│         ▼                                                  ▼               │
│  ┌─────────────────┐                            ┌─────────────────┐        │
│  │   WebSocket     │                            │    REST API     │        │
│  │   Gateway       │◄─────── gRPC ─────────────►│    Service      │        │
│  │  (Go+Gorilla)   │                            │    (Go+Gin)     │        │
│  └────────┬────────┘                            └────────┬────────┘        │
│           │                                              │                  │
│           ▼                                              ▼                  │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                      MESSAGE BROKER (Kafka)                         │   │
│  │   ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────┐  │   │
│  │   │messages.send │ │messages.recv │ │notifications │ │ presence │  │   │
│  │   └──────────────┘ └──────────────┘ └──────────────┘ └──────────┘  │   │
│  └─────────────────────────────┬───────────────────────────────────────┘   │
│                                ▼                                            │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐    │
│  │  Message  │ │   User    │ │  Channel  │ │  Search   │ │   Push    │    │
│  │  Service  │ │  Service  │ │  Service  │ │  Service  │ │  Service  │    │
│  │   (Go)    │ │   (Go)    │ │   (Go)    │ │   (Go)    │ │   (Go)    │    │
│  └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └─────┬─────┘    │
├────────┼─────────────┼─────────────┼─────────────┼─────────────┼──────────┤
│                              DATA LAYER                                     │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐    │
│  │ Cassandra │ │PostgreSQL │ │   Redis   │ │  Elastic  │ │    S3     │    │
│  │ (Messages)│ │  (Users)  │ │ (Cache/   │ │ (Search)  │ │ (Media)   │    │
│  │           │ │           │ │  Presence)│ │           │ │           │    │
│  └───────────┘ └───────────┘ └───────────┘ └───────────┘ └───────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Core Components

### 3.1 WebSocket Gateway Service

The WebSocket Gateway is the primary entry point for real-time communication. It manages persistent connections with clients and handles message routing.

**Responsibilities:**

- Maintain persistent WebSocket connections with clients
- Authenticate connections via JWT tokens
- Route incoming messages to appropriate Kafka topics
- Deliver messages from Kafka to connected clients
- Track user presence (online/offline/typing)
- Handle heartbeat/ping-pong for connection health

**Technology:** Go with Gorilla WebSocket library

### 3.2 Message Service

Handles all message-related operations including persistence, retrieval, and processing.

**Responsibilities:**

- Validate and process incoming messages
- Generate unique message IDs using Snowflake algorithm
- Persist messages to Cassandra
- Handle message status updates (sent, delivered, read)
- Process message edits and deletions
- Index messages for search

### 3.3 User Service

Manages user accounts, authentication, profiles, and relationships.

**Responsibilities:**

- User registration and authentication (OAuth, Email/Password)
- Profile management and settings
- Contact/friend list management
- User blocking and privacy settings
- JWT token generation and validation

### 3.4 Channel Service

Manages chat channels, groups, and direct message threads.

**Responsibilities:**

- Create and manage channels (public, private, DM)
- Member management and permissions
- Channel metadata and settings
- Pin messages, channel topics

### 3.5 Presence Service

Tracks and broadcasts user presence information in real-time.

**Responsibilities:**

- Track online/offline/away status
- Handle typing indicators
- Last seen timestamps
- Custom status messages

---

## 4. Data Flow: Message Send/Receive

### 4.1 Message Sending Flow

When a user sends a message, the following sequence occurs:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    MESSAGE SENDING SEQUENCE DIAGRAM                         │
├─────────────────────────────────────────────────────────────────────────────┤
│  Client          WS Gateway       Kafka         Message Svc    Cassandra   │
│    │                 │               │               │             │        │
│    │──1.Send Msg────►│               │               │             │        │
│    │                 │               │               │             │        │
│    │◄─2.Ack(temp_id)─│               │               │             │        │
│    │                 │               │               │             │        │
│    │                 │──3.Publish───►│               │             │        │
│    │                 │               │               │             │        │
│    │                 │               │──4.Consume───►│             │        │
│    │                 │               │               │             │        │
│    │                 │               │               │──5.Write───►│        │
│    │                 │               │               │             │        │
│    │                 │               │◄─6.Pub Recv───│             │        │
│    │                 │               │               │             │        │
│    │                 │◄─7.Consume────│               │             │        │
│    │                 │               │               │             │        │
│    │◄─8.Deliver──────│               │               │             │        │
│    │  (final_id)     │               │               │             │        │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Detailed Steps:**

1. **Client sends message:** User composes message and sends via WebSocket with a temporary client-generated ID (UUID)

2. **Gateway acknowledges:** WebSocket Gateway immediately returns acknowledgment with temp_id to confirm receipt (optimistic UI update)

3. **Publish to Kafka:** Gateway publishes message to `messages.send` topic, partitioned by channel_id for ordering

4. **Message Service consumes:** Message Service processes the event, validates content, generates Snowflake ID

5. **Persist to Cassandra:** Message is written to Cassandra with QUORUM consistency for durability

6. **Publish delivery event:** Message Service publishes to `messages.receive` topic with routing info for recipients

7. **Gateway consumes:** WebSocket Gateway(s) consume from `messages.receive` based on connected users

8. **Deliver to recipients:** Gateway pushes message to all online recipients via their WebSocket connections

### 4.2 Message Receiving Flow (Offline Users)

For offline users, the system handles message delivery through push notifications and sync mechanisms:

1. Message Service detects recipient is offline (no active WebSocket connection in Presence Service)
2. Publishes event to `notifications` Kafka topic
3. Push Notification Service sends FCM/APNs notification
4. When user reconnects, client requests sync from last known message_id
5. Message Service returns all messages since that ID from Cassandra

---

## 5. Technology Choices & Rationale

### 5.1 Why Golang

Golang is the ideal choice for this real-time chat architecture for several compelling reasons:

| Feature | Benefit for Chat Application |
|---------|------------------------------|
| **Goroutines & Channels** | Handle 100K+ concurrent WebSocket connections per server with minimal memory (~2KB per goroutine vs ~1MB per thread). Native concurrency model is perfect for managing many simultaneous connections. |
| **Fast Compilation** | Rapid iteration during development, quick CI/CD pipelines. Single binary deployment simplifies container images and reduces operational complexity. |
| **Low Latency GC** | Sub-millisecond garbage collection pauses (since Go 1.8) ensure consistent message delivery latency without stop-the-world pauses. |
| **Strong Networking** | Excellent standard library for HTTP/2, WebSocket (gorilla/websocket), and gRPC. Built-in support for connection pooling and efficient I/O multiplexing. |
| **Static Typing + Safety** | Compile-time error detection, easier refactoring, and better tooling support. Strong typing prevents runtime errors in production. |
| **Ecosystem** | Rich ecosystem for chat: Kafka clients (sarama, confluent-kafka-go), Redis (go-redis), Cassandra (gocql), and observability (prometheus, jaeger). |

### 5.2 Database Choices

#### Apache Cassandra - Message Storage

**Why Cassandra for messages:**

- **Write-optimized:** Cassandra excels at high-volume writes which is critical for chat where write:read ratio is typically 10:1
- **Linear scalability:** Add nodes to scale horizontally without downtime. Handles billions of messages across hundreds of nodes
- **Time-series optimized:** Partition by channel_id, cluster by timestamp - perfect for "get messages since X" queries
- **Tunable consistency:** Use QUORUM writes for durability, ONE reads for speed with eventual consistency
- **No SPOF:** Masterless architecture means any node can accept reads/writes

#### PostgreSQL - User & Channel Data

**Why PostgreSQL for users/channels:**

- **ACID compliance:** Strong consistency for user accounts, permissions, and channel membership - data that must be accurate
- **Complex queries:** Rich query capabilities for user search, permission checks, and relationship graphs
- **Foreign keys:** Enforce referential integrity between users, channels, and memberships
- **Mature ecosystem:** Excellent tooling, replication (streaming, logical), and extensions (pg_partman, pgvector)

#### Redis - Caching & Presence

**Why Redis for cache/presence:**

- **Sub-millisecond latency:** In-memory storage provides microsecond response times for hot data
- **Pub/Sub:** Native pub/sub for cross-server presence updates and typing indicators
- **Data structures:** Sets for online users per channel, sorted sets for typing indicators with TTL
- **Session storage:** Store WebSocket server assignments: user_id → server_id mapping
- **Rate limiting:** Token bucket implementation for API rate limiting

### 5.3 Communication Methods

#### WebSocket - Client Communication

**Why WebSocket:**

- **Full-duplex:** Bidirectional communication over single TCP connection - essential for real-time messaging
- **Low overhead:** 2-byte frame header vs 500+ bytes for HTTP headers - critical at scale
- **Push capability:** Server can push messages instantly without client polling
- **Universal support:** Works in all browsers, mobile, and desktop clients

#### Apache Kafka - Event Streaming

**Why Kafka:**

- **Durability:** Messages persisted to disk with replication - no data loss even if consumers fail
- **Ordering guarantees:** Messages within a partition are strictly ordered - partition by channel_id
- **Consumer groups:** Multiple WebSocket servers can form consumer groups for parallel processing
- **Replay capability:** Consumers can replay from any offset - useful for recovery and debugging
- **Throughput:** Handle millions of messages/second with horizontal scaling

#### gRPC - Internal Service Communication

**Why gRPC:**

- **Protocol Buffers:** Binary serialization is 3-10x smaller and faster than JSON
- **HTTP/2:** Multiplexing, header compression, and connection reuse
- **Streaming:** Bidirectional streaming for presence updates between services
- **Code generation:** Auto-generate Go client/server code from .proto definitions
- **Strong typing:** Compile-time type checking across service boundaries

---

## 6. Scalability Strategy

### 6.1 Horizontal Scaling Architecture

Every component in the architecture is designed for horizontal scaling without single points of failure.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      HORIZONTAL SCALING TOPOLOGY                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Region: US-East                        Region: EU-West                     │
│  ┌─────────────────────────┐            ┌─────────────────────────┐        │
│  │    Load Balancer        │◄──────────►│    Load Balancer        │        │
│  └───────────┬─────────────┘            └───────────┬─────────────┘        │
│              │                                      │                       │
│  ┌───────────┴───────────┐              ┌───────────┴───────────┐          │
│  │ WS-1 │ WS-2 │ WS-N    │              │ WS-1 │ WS-2 │ WS-N    │          │
│  │100K  │100K  │100K     │              │100K  │100K  │100K     │          │
│  │conns │conns │conns    │              │conns │conns │conns    │          │
│  └───────────┬───────────┘              └───────────┬───────────┘          │
│              │                                      │                       │
│              ▼                                      ▼                       │
│  ┌─────────────────────────────────────────────────────────────────┐       │
│  │              Kafka Cluster (Multi-Region Replication)           │       │
│  │   ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐   │       │
│  │   │Broker 1│  │Broker 2│  │Broker 3│  │Broker 4│  │Broker N│   │       │
│  │   └────────┘  └────────┘  └────────┘  └────────┘  └────────┘   │       │
│  └─────────────────────────────────────────────────────────────────┘       │
│                                                                             │
│              ▼                                      ▼                       │
│  ┌─────────────────────────────────────────────────────────────────┐       │
│  │           Cassandra Cluster (Multi-DC Replication)              │       │
│  │   DC: US-East (RF=3)              DC: EU-West (RF=3)            │       │
│  │   ┌────┐┌────┐┌────┐             ┌────┐┌────┐┌────┐            │       │
│  │   │N1  ││N2  ││N3  │◄───────────►│N1  ││N2  ││N3  │            │       │
│  │   └────┘└────┘└────┘             └────┘└────┘└────┘            │       │
│  └─────────────────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.2 Scaling Strategies by Component

| Component | Scaling Strategy | Target Capacity |
|-----------|------------------|-----------------|
| **WebSocket Gateway** | Horizontal pod autoscaling based on connection count. Each pod handles ~100K connections. Sticky sessions via consistent hashing. | 10M+ concurrent connections |
| **Message Service** | Kafka consumer group with partitions = 3x number of pods. Scale based on message throughput and consumer lag. | 1M+ messages/second |
| **Cassandra** | Add nodes to cluster. Data automatically rebalances. Partition by channel_id with time-bucketed clustering. | Petabytes of message history |
| **Redis Cluster** | Redis Cluster mode with 16384 hash slots. Add master nodes for capacity, replicas for read scaling. | Sub-ms latency at scale |
| **Kafka** | Increase partitions for parallelism. Add brokers for throughput. MirrorMaker 2 for cross-region replication. | Millions of events/second |

---

## 7. Fault Tolerance & Reliability

### 7.1 Failure Scenarios & Recovery

#### WebSocket Server Crash

**Detection:** Health checks fail within 10 seconds

**Impact:** ~100K users disconnected

**Recovery:**

1. Load balancer removes unhealthy node within 10s
2. Clients detect disconnect via heartbeat timeout (30s)
3. Clients reconnect with exponential backoff (1s, 2s, 4s...)
4. New server assigned via consistent hashing
5. Client requests message sync from last known message_id

**Total recovery time:** < 45 seconds

#### Kafka Broker Failure

**Prevention:** `min.insync.replicas=2`, `replication.factor=3`

**Recovery:**

1. Controller detects broker failure via ZooKeeper session timeout
2. Leader election for affected partitions (~5s)
3. Producers/consumers reconnect to new leaders
4. Under-replicated partitions rebalance when broker returns

**Impact:** Brief latency spike (~5s), zero message loss

#### Cassandra Node Failure

**Prevention:** Replication Factor = 3 per datacenter

**Recovery:**

1. Gossip protocol detects failure within 10s
2. Requests automatically routed to replica nodes
3. Hinted handoff stores writes for failed node
4. When node recovers, hinted handoff replays missed writes
5. Anti-entropy repair ensures consistency

**Impact:** Zero downtime with QUORUM consistency

### 7.2 Data Durability Guarantees

- **Messages:** Written to Kafka with `acks=all` before acknowledgment. Replicated to Cassandra with QUORUM consistency.
- **User data:** PostgreSQL with synchronous streaming replication to standby.
- **Media files:** S3 with cross-region replication (99.999999999% durability).

---

## 8. Load Balancing Strategy

### 8.1 WebSocket Load Balancing

WebSocket connections require special load balancing considerations due to their persistent nature.

**Layer 7 Load Balancer Configuration:**

- **Connection affinity:** Use consistent hashing based on user_id to ensure reconnections go to same server when possible
- **Health checks:** Active WebSocket ping every 5s, HTTP /health endpoint for LB
- **Connection draining:** 30-minute drain period before removing server, allowing graceful handoff
- **Timeout configuration:** `idle_timeout=3600s` for long-lived connections

### 8.2 Connection Distribution Algorithm

```go
// Consistent Hashing for WebSocket Server Selection
func selectServer(userID string, servers []Server) Server {
    hash := fnv.New32a()
    hash.Write([]byte(userID))
    
    // Find server on hash ring
    hashValue := hash.Sum32()
    
    // Virtual nodes for better distribution
    return ring.GetNode(hashValue)
}
```

### 8.3 Cross-Region Load Balancing

- **GeoDNS:** Route users to nearest region based on IP geolocation
- **Anycast:** Global anycast IPs route to nearest edge location
- **Failover:** Automatic failover to secondary region if primary becomes unhealthy

---

## 9. Message Consistency & Ordering

### 9.1 Ordering Guarantees

- **Within a channel:** Strict ordering guaranteed via Kafka partition per channel
- **Across channels:** No ordering guarantee (not needed)
- **Delivery semantics:** At-least-once with idempotency keys for exactly-once semantics

### 9.2 Message ID Generation (Snowflake)

We use Twitter's Snowflake algorithm for globally unique, time-ordered message IDs:

```
┌───────────────────────────────────────────────────────────────────┐
│  Snowflake ID Structure (64-bit)                                  │
├───────────────────────────────────────────────────────────────────┤
│  [1 bit unused][41 bits timestamp][10 bits node][12 bits seq]     │
│                                                                   │
│  Timestamp: ms since epoch (69 years range)                       │
│  Node ID: Up to 1024 generator nodes                              │
│  Sequence: 4096 IDs per ms per node                               │
│                                                                   │
│  Capacity: 4M IDs/second per node, ~4B IDs/second total           │
└───────────────────────────────────────────────────────────────────┘
```

### 9.3 Conflict Resolution

For concurrent edits and offline sync scenarios:

- **Last-write-wins:** Based on Snowflake timestamp for message edits
- **Vector clocks:** For complex conflict detection in collaborative features
- **Idempotency keys:** Client-generated UUID prevents duplicate message creation

---

## 10. Database Schema Design

### 10.1 Cassandra Schema (Messages)

```sql
-- Messages table: Optimized for reading messages by channel
CREATE TABLE messages (
    channel_id      UUID,
    bucket          TEXT,        -- Time bucket: 'YYYY-MM' for partition size control
    message_id      BIGINT,      -- Snowflake ID (time-ordered)
    sender_id       UUID,
    content         TEXT,
    content_type    TEXT,        -- 'text', 'image', 'file', 'reply'
    metadata        TEXT,        -- JSON: attachments, mentions, reactions
    reply_to        BIGINT,      -- Parent message_id for threads
    edited_at       TIMESTAMP,
    deleted         BOOLEAN,
    created_at      TIMESTAMP,
    PRIMARY KEY ((channel_id, bucket), message_id)
) WITH CLUSTERING ORDER BY (message_id DESC);

-- Message delivery status (per recipient)
CREATE TABLE message_status (
    user_id         UUID,
    channel_id      UUID,
    last_read_id    BIGINT,      -- Last message_id user has read
    last_delivered  BIGINT,      -- Last message_id delivered to client
    unread_count    INT,
    PRIMARY KEY (user_id, channel_id)
);
```

### 10.2 PostgreSQL Schema (Users & Channels)

```sql
-- Users table
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) UNIQUE NOT NULL,
    username        VARCHAR(50) UNIQUE NOT NULL,
    password_hash   VARCHAR(255),
    display_name    VARCHAR(100),
    avatar_url      VARCHAR(500),
    status          VARCHAR(20) DEFAULT 'offline',
    status_message  VARCHAR(200),
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

-- Channels table
CREATE TABLE channels (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100),
    type            VARCHAR(20) NOT NULL, -- 'dm', 'group', 'public'
    creator_id      UUID REFERENCES users(id),
    avatar_url      VARCHAR(500),
    description     TEXT,
    created_at      TIMESTAMP DEFAULT NOW()
);

-- Channel membership (many-to-many)
CREATE TABLE channel_members (
    channel_id      UUID REFERENCES channels(id) ON DELETE CASCADE,
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    role            VARCHAR(20) DEFAULT 'member', -- 'owner', 'admin', 'member'
    joined_at       TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (channel_id, user_id)
);

-- Indexes
CREATE INDEX idx_channel_members_user ON channel_members(user_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
```

---

## 11. API Design

### 11.1 WebSocket API

All real-time communication uses a JSON-based WebSocket protocol:

```json
// Client -> Server: Send Message
{
  "type": "message.send",
  "id": "client-uuid-123",
  "payload": {
    "channel_id": "uuid",
    "content": "Hello world!",
    "content_type": "text",
    "reply_to": null
  }
}

// Server -> Client: Message Acknowledgment
{
  "type": "message.ack",
  "id": "client-uuid-123",
  "payload": {
    "status": "received"
  }
}

// Server -> Client: New Message
{
  "type": "message.new",
  "payload": {
    "message_id": "1234567890123456789",
    "client_id": "client-uuid-123",
    "channel_id": "uuid",
    "sender": {
      "id": "uuid",
      "username": "john",
      "avatar_url": "https://..."
    },
    "content": "Hello world!",
    "created_at": "2024-01-15T10:30:00Z"
  }
}

// Client -> Server: Typing Indicator
{
  "type": "typing.start",
  "payload": {
    "channel_id": "uuid"
  }
}

// Server -> Client: User Typing
{
  "type": "typing.update",
  "payload": {
    "channel_id": "uuid",
    "users": ["user_id_1", "user_id_2"]
  }
}

// Client -> Server: Mark as Read
{
  "type": "message.read",
  "payload": {
    "channel_id": "uuid",
    "message_id": "1234567890123456789"
  }
}
```

### 11.2 REST API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/auth/register` | POST | Register new user |
| `/api/v1/auth/login` | POST | Authenticate user, return JWT token |
| `/api/v1/auth/refresh` | POST | Refresh JWT token |
| `/api/v1/auth/logout` | POST | Invalidate refresh token |
| `/api/v1/users/me` | GET | Get current user profile |
| `/api/v1/users/me` | PATCH | Update current user profile |
| `/api/v1/users/:id` | GET | Get user by ID |
| `/api/v1/users/search` | GET | Search users by username/email |
| `/api/v1/channels` | GET | List user's channels |
| `/api/v1/channels` | POST | Create new channel |
| `/api/v1/channels/:id` | GET | Get channel details |
| `/api/v1/channels/:id` | PATCH | Update channel |
| `/api/v1/channels/:id` | DELETE | Delete channel |
| `/api/v1/channels/:id/messages` | GET | Get message history (paginated) |
| `/api/v1/channels/:id/members` | GET | List channel members |
| `/api/v1/channels/:id/members` | POST | Add member to channel |
| `/api/v1/channels/:id/members/:userId` | DELETE | Remove member |
| `/api/v1/upload` | POST | Upload media file (returns CDN URL) |
| `/api/v1/upload/:id` | DELETE | Delete uploaded file |

---

## 12. Monitoring & Observability

### 12.1 Key Metrics

| Metric Category | Key Metrics | Alert Threshold |
|-----------------|-------------|-----------------|
| **Connection Health** | Active WebSocket connections, Connection rate, Disconnect rate | Disconnect rate > 5%/min |
| **Message Latency** | P50/P95/P99 end-to-end latency, Kafka consumer lag | P99 > 500ms, Lag > 10K |
| **Throughput** | Messages/second, API requests/second | < 80% of capacity |
| **Error Rates** | Failed deliveries, API errors (4xx/5xx) | Error rate > 1% |
| **Resource Usage** | CPU, Memory, Disk I/O, Network | CPU > 80%, Memory > 85% |

### 12.2 Observability Stack

- **Metrics:** Prometheus + Grafana for time-series metrics and dashboards
- **Logging:** ELK Stack (Elasticsearch, Logstash, Kibana) with structured JSON logs
- **Tracing:** Jaeger for distributed tracing across microservices
- **Alerting:** PagerDuty integration with Prometheus Alertmanager

### 12.3 Key Dashboards

1. **Real-time Overview:** Active connections, message rate, latency percentiles
2. **Service Health:** Per-service CPU/memory, error rates, request latency
3. **Kafka Monitoring:** Consumer lag, partition distribution, throughput
4. **Database Health:** Query latency, connection pools, replication lag

---

## 13. Security Considerations

### 13.1 Authentication & Authorization

- **JWT Tokens:** Short-lived access tokens (15 min) + long-lived refresh tokens (7 days)
- **WebSocket Auth:** Token passed in initial connection handshake, validated before upgrade
- **Channel Permissions:** RBAC with roles: owner, admin, member, guest
- **OAuth 2.0:** Support for Google, Apple, GitHub SSO

### 13.2 Data Protection

- **Encryption in Transit:** TLS 1.3 for all connections (WebSocket WSS, HTTPS)
- **Encryption at Rest:** AES-256 encryption for all database storage
- **E2E Encryption:** Optional end-to-end encryption for DMs using Signal Protocol
- **PII Handling:** Separate encryption keys for PII, audit logging for access

### 13.3 Rate Limiting & Abuse Prevention

- **API Rate Limits:** Token bucket algorithm: 100 requests/minute per user
- **Message Rate Limits:** 10 messages/second per user, 1000/minute per channel
- **Connection Limits:** Max 5 simultaneous connections per user
- **Content Moderation:** Async content scanning for spam, malware, abuse

### 13.4 Security Headers

```
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Content-Security-Policy: default-src 'self'
X-XSS-Protection: 1; mode=block
```

---

## 14. Deployment Architecture

### 14.1 Kubernetes Deployment

All services are containerized and deployed on Kubernetes for orchestration, auto-scaling, and self-healing capabilities.

```yaml
# WebSocket Gateway Deployment (HPA enabled)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: websocket-gateway
  namespace: chat
spec:
  replicas: 10  # Base replicas
  selector:
    matchLabels:
      app: websocket-gateway
  template:
    metadata:
      labels:
        app: websocket-gateway
    spec:
      containers:
      - name: gateway
        image: chat/ws-gateway:v1.2.0
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 3
        env:
        - name: KAFKA_BROKERS
          valueFrom:
            configMapKeyRef:
              name: chat-config
              key: kafka-brokers
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: chat-secrets
              key: redis-url
---
# Horizontal Pod Autoscaler
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: websocket-gateway-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: websocket-gateway
  minReplicas: 10
  maxReplicas: 100
  metrics:
  - type: Pods
    pods:
      metric:
        name: websocket_connections
      target:
        type: AverageValue
        averageValue: "80000"  # Scale at 80K connections per pod
```

### 14.2 CI/CD Pipeline

- **Source Control:** GitHub with protected main branch, required PR reviews
- **CI:** GitHub Actions for build, test, lint, security scan
- **CD:** ArgoCD for GitOps-based Kubernetes deployments
- **Rollout Strategy:** Canary deployments with automatic rollback on error spike

```yaml
# GitHub Actions Workflow
name: CI/CD Pipeline
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    - run: go test -v -race -coverprofile=coverage.out ./...
    - run: go vet ./...
    - uses: golangci/golangci-lint-action@v3

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: docker/build-push-action@v4
      with:
        push: true
        tags: chat/ws-gateway:${{ github.sha }}

  deploy:
    needs: build
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
    - name: Update ArgoCD Application
      run: |
        # Update image tag in GitOps repo
        # ArgoCD auto-syncs the change
```

---

## 15. Summary

This architecture provides a robust, scalable foundation for a real-time chat application capable of handling millions of concurrent users with sub-100ms message delivery latency.

### Key Architecture Decisions

| Decision | Rationale |
|----------|-----------|
| **Golang** | Excellent concurrency model (goroutines), low latency GC, strong networking libraries |
| **Event-Driven (Kafka)** | Durable, ordered message streaming with decoupled services |
| **Polyglot Persistence** | Cassandra for messages (write-heavy), PostgreSQL for users (ACID), Redis for cache/presence |
| **Microservices** | Independent scaling and deployment of each component |
| **Multi-Region** | Geographic distribution for low latency and disaster recovery |

### Capacity Targets

| Metric | Target |
|--------|--------|
| Concurrent WebSocket connections | 10M+ |
| Message throughput | 1M+ messages/second |
| Message delivery latency (P99) | < 100ms |
| Availability SLA | 99.99% |

### Technology Stack Summary

```
┌─────────────────────────────────────────────────────────────┐
│                    TECHNOLOGY STACK                         │
├─────────────────────────────────────────────────────────────┤
│  Language:        Go 1.21+                                  │
│  Web Framework:   Gin (REST), Gorilla (WebSocket)           │
│  RPC:             gRPC + Protocol Buffers                   │
│  Message Broker:  Apache Kafka                              │
│  Databases:       Cassandra, PostgreSQL, Redis              │
│  Search:          Elasticsearch                             │
│  Storage:         AWS S3                                    │
│  Container:       Docker + Kubernetes                       │
│  CI/CD:           GitHub Actions + ArgoCD                   │
│  Monitoring:      Prometheus + Grafana + Jaeger             │
│  Load Balancer:   NGINX / AWS ALB                           │
│  CDN:             CloudFlare                                │
└─────────────────────────────────────────────────────────────┘
```

---

*End of Document*
