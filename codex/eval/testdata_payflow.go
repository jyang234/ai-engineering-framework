package eval

// PayFlowDocuments returns 30 documents about the fictional PayFlow payment
// processing system. Documents cross-reference each other and contain
// realistic technical content spanning ADRs, architecture docs, design docs,
// meeting notes, API documentation, and code patterns.
func PayFlowDocuments() []TestDocument {
	return []TestDocument{
		// =====================================================================
		// ADR-001 through ADR-006: Architecture Decision Records
		// =====================================================================
		{
			ID:    "adr-001",
			Type:  "decision",
			Title: "ADR-001: Event Sourcing for Payment State Management",
			Scope: "project",
			Tags:  []string{"event-sourcing", "payment-state", "architecture", "cqrs"},
			Content: `# ADR-001: Event Sourcing for Payment State Management

## Status
Accepted

## Context
PayFlow processes millions of payment transactions daily across multiple payment providers (Stripe, Adyen, Braintree). The current state-based model makes it difficult to audit payment lifecycle transitions, reconstruct historical state for dispute resolution, and maintain consistency across distributed services. The payment processing pipeline (see arch-002) requires reliable state tracking through complex multi-step workflows involving authorization, capture, settlement, and potential chargebacks.

Our fraud detection system (arch-004) also needs access to the full history of state transitions to identify suspicious patterns, such as rapid authorization-void cycles or repeated partial captures.

## Decision
We will adopt event sourcing for all payment state management. Each payment will be represented as an ordered sequence of domain events (PaymentInitiated, PaymentAuthorized, PaymentCaptured, PaymentSettled, PaymentRefunded, PaymentDisputed, etc.) stored in an append-only event store. The current state of any payment will be derived by replaying its event stream.

We will implement the Event Sourcing Aggregate pattern (see pattern-003) to encapsulate business rules for valid state transitions. The aggregate root will be the Payment entity, which validates transitions before emitting events.

Read models will be maintained through event projections, allowing optimized query patterns for merchant dashboards, settlement reconciliation, and reporting. The CQRS separation ensures write-path performance is not impacted by complex query requirements.

## Consequences
- Full audit trail for every payment state change, critical for PCI compliance (see meeting-002)
- Ability to reconstruct payment state at any point in time for dispute resolution
- Natural fit for distributed saga orchestration (see pattern-004) across payment providers
- Increased storage requirements (mitigated by event compaction for aged payments)
- Development team needs training on event sourcing patterns
- Eventual consistency between write and read models requires careful UX design (see design-005 for status tracking approach)

## Related
- arch-002: Payment Processing Pipeline
- pattern-003: Event Sourcing Aggregate Pattern
- pattern-004: Saga Orchestration for Distributed Transactions
- design-005: Payment Status Tracking`,
		},
		{
			ID:    "adr-002",
			Type:  "decision",
			Title: "ADR-002: PostgreSQL for Transaction Store over DynamoDB",
			Scope: "project",
			Tags:  []string{"postgresql", "dynamodb", "database", "transaction-store", "infrastructure"},
			Content: `# ADR-002: PostgreSQL for Transaction Store over DynamoDB

## Status
Accepted

## Context
PayFlow needs a durable store for payment transaction records, event streams (per ADR-001), and merchant account data. We evaluated PostgreSQL (with Citus for horizontal scaling) and DynamoDB for this role. Key requirements include ACID transactions for financial data integrity, support for complex queries required by the settlement engine (arch-003), sub-10ms p99 read latency for payment status lookups (api-002), and the ability to handle 50,000 writes per second during peak periods such as Black Friday (see meeting-003).

DynamoDB offers seamless horizontal scaling and single-digit millisecond latency, but its query model is restrictive for the settlement reconciliation workflows that require joins across payment events, merchant accounts, and provider settlement files. The settlement engine (arch-003) performs complex aggregations that would require extensive denormalization or secondary index abuse in DynamoDB.

## Decision
We will use PostgreSQL 16 with the Citus extension for horizontal sharding as the primary transaction store. Payment records will be sharded by merchant_id to ensure related transactions are co-located. The event store tables will use partitioning by created_at for efficient time-range queries and archival.

For hot-path payment status lookups (api-002), we will maintain a Redis cache layer with a 30-second TTL, populated by event projections from the event sourcing system.

Connection pooling via PgBouncer will handle the high connection count from our microservices. We will use logical replication to feed read replicas for the reporting and analytics workloads.

## Consequences
- ACID guarantees for financial transaction integrity
- Rich query support for settlement reconciliation (arch-003) and fraud detection (arch-004)
- Operational complexity of managing PostgreSQL clusters and Citus sharding
- Need for careful capacity planning for Black Friday traffic spikes (meeting-003)
- Redis cache adds a consistency window for status reads (acceptable per design-005)
- Team has strong PostgreSQL expertise, reducing operational risk

## Related
- ADR-001: Event Sourcing for Payment State Management
- arch-003: Settlement Engine
- arch-004: Fraud Detection System
- meeting-003: Performance Review - Black Friday Prep`,
		},
		{
			ID:    "adr-003",
			Type:  "decision",
			Title: "ADR-003: Idempotency Keys for Payment APIs",
			Scope: "project",
			Tags:  []string{"idempotency", "api-design", "reliability", "payment-api"},
			Content: `# ADR-003: Idempotency Keys for Payment APIs

## Status
Accepted

## Context
Payment APIs are inherently dangerous to retry without idempotency guarantees. Network timeouts, client crashes, and load balancer failovers can cause merchants to re-send payment creation requests, potentially resulting in duplicate charges. This is the single most critical reliability concern for PayFlow, as duplicate payments directly impact merchant trust and trigger costly chargeback processes.

The Payment API v2 design (design-001) requires a robust idempotency mechanism that works across all payment operations: creation, capture, refund, and void. The pattern must be compatible with our event sourcing architecture (ADR-001) and the distributed saga orchestration (pattern-004) used for multi-provider payment routing.

## Decision
All mutating payment API endpoints will require an Idempotency-Key header (see api-001 for the create payment endpoint specification). The idempotency implementation follows the pattern described in pattern-001.

Key design choices:
1. Idempotency keys are scoped to the merchant (merchant_id + idempotency_key composite uniqueness) and expire after 24 hours.
2. The idempotency record stores the full request hash, response status, and response body. Subsequent requests with the same key but different request bodies return HTTP 422.
3. Idempotency records are stored in PostgreSQL (per ADR-002) in the same transaction as the payment event, ensuring atomicity.
4. For in-flight requests (key exists but no response recorded), the API returns HTTP 409 Conflict with a Retry-After header.
5. The idempotency check occurs before any side effects, including provider API calls.

The implementation uses a PostgreSQL advisory lock on the idempotency key hash to prevent concurrent processing of duplicate requests, which is simpler and more reliable than distributed locks.

## Consequences
- Merchants can safely retry payment requests without risk of duplicate charges
- Slight increase in latency (~2ms) for the advisory lock acquisition
- 24-hour expiry balances storage costs against merchant retry patterns
- API documentation (api-001) must clearly communicate idempotency key requirements
- The webhook delivery system (design-002) uses separate idempotency for outbound delivery

## Related
- pattern-001: Idempotent Request Processing
- api-001: Create Payment Endpoint
- design-001: Payment API v2
- ADR-001: Event Sourcing for Payment State`,
		},
		{
			ID:    "adr-004",
			Type:  "decision",
			Title: "ADR-004: Circuit Breaker for Payment Provider Integration",
			Scope: "project",
			Tags:  []string{"circuit-breaker", "resilience", "payment-providers", "fault-tolerance"},
			Content: `# ADR-004: Circuit Breaker for Payment Provider Integration

## Status
Accepted

## Context
PayFlow integrates with multiple payment providers (Stripe, Adyen, Braintree, Worldpay) for payment processing. During the November 2024 incident (meeting-004), a 45-minute Stripe API degradation caused cascading failures across our payment pipeline. Requests to Stripe were timing out at 30 seconds, exhausting our connection pool, and blocking payments that could have been routed to healthy providers.

The payment processing pipeline (arch-002) routes transactions based on provider capabilities, merchant preferences, and cost optimization. When a provider degrades, we need to fail fast and route to alternatives rather than accumulating timeouts.

## Decision
We will implement the circuit breaker pattern (see pattern-002 for implementation details) for all payment provider integrations. Each provider integration will have an independent circuit breaker with the following configuration:

- Failure threshold: 5 consecutive failures or >50% failure rate in a 60-second window
- Open state duration: 30 seconds before attempting a half-open probe
- Half-open: Allow 3 probe requests; if 2 succeed, close the circuit
- Monitored failures: HTTP 5xx responses, connection timeouts, and request timeouts (>5s)
- Excluded from failure count: HTTP 4xx (client errors), rate limiting (HTTP 429 with Retry-After)

When a provider circuit opens, the payment pipeline (arch-002) will automatically route eligible transactions to the next preferred provider in the merchant's routing configuration. Transactions that cannot be rerouted (e.g., captures on existing authorizations) will be queued for retry when the circuit closes.

Circuit state changes will be published as operational events for alerting and the merchant dashboard. The fraud detection system (arch-004) will be notified of routing changes to adjust risk scoring (provider switch can be a fraud signal if not correlated with circuit events).

## Consequences
- Fast failure and automatic rerouting during provider outages
- Prevents cascade failures from propagating across the payment pipeline
- Merchants may see temporary routing to non-preferred providers (impacts interchange rates)
- Queued transactions during open circuits need TTL-based expiry and merchant notification
- Circuit state must be shared across service instances (stored in Redis)
- Operational dashboards need circuit state visibility per provider

## Related
- pattern-002: Circuit Breaker Implementation
- arch-002: Payment Processing Pipeline
- arch-004: Fraud Detection System
- meeting-004: Incident Review - Payment Failures`,
		},
		{
			ID:    "adr-005",
			Type:  "decision",
			Title: "ADR-005: PCI DSS Tokenization Strategy",
			Scope: "project",
			Tags:  []string{"pci-dss", "tokenization", "security", "compliance", "card-data"},
			Content: `# ADR-005: PCI DSS Tokenization Strategy

## Status
Accepted

## Context
PayFlow must achieve PCI DSS Level 1 compliance to process card-present and card-not-present transactions directly. The current architecture requires that raw cardholder data (PAN, CVV, expiration) never touches our application servers. The PCI compliance review (meeting-002) identified that minimizing the cardholder data environment (CDE) scope is critical to reducing audit complexity and operational risk.

Our merchant onboarding flow (design-003) collects payment method information via client-side tokenization, but the current implementation has inconsistencies across web, mobile, and server-to-server integration paths.

## Decision
We will implement a unified tokenization strategy with the following components:

1. Client-Side Tokenization: All cardholder data is tokenized in the client (browser or mobile SDK) before reaching PayFlow servers. We will provide PayFlow.js (browser) and PayFlow Mobile SDK (iOS/Android) that communicate directly with our PCI-scoped tokenization service.

2. Token Vault: A dedicated, isolated microservice running in a PCI-scoped network segment. The vault stores encrypted PANs using AES-256-GCM with per-merchant encryption keys managed by AWS KMS. Tokens are format-preserving (maintain last-four digits and BIN for routing).

3. Token Lifecycle: Tokens are bound to a merchant and have configurable TTL. Single-use tokens (for one-time payments) expire after 15 minutes. Multi-use tokens (for subscriptions and saved cards) persist until explicitly deleted or the card expires.

4. Network Tokenization: For supported card networks (Visa, Mastercard), we will request network tokens through the token vault, replacing PANs with network-level tokens. This improves authorization rates by 2-3% and reduces the PCI scope further.

5. Detokenization: Only the payment processing pipeline (arch-002) can request detokenization, and only for the duration of a provider API call. Detokenized data is held in memory only and never logged or persisted outside the vault.

The authentication system (api-005) enforces that only authorized services can access tokenization and detokenization endpoints using mutual TLS and short-lived JWT tokens.

## Consequences
- PayFlow application servers are outside the CDE, dramatically reducing PCI audit scope
- Client-side tokenization provides a consistent integration experience across all channels
- Token vault becomes a critical single point of failure (mitigated by multi-AZ deployment)
- Network tokens improve auth rates but add complexity to token lifecycle management
- Merchant onboarding (design-003) must provision SDK credentials and configure token policies

## Related
- meeting-002: PCI Compliance Review
- design-003: Merchant Onboarding Flow
- arch-002: Payment Processing Pipeline
- api-005: Authentication and Authorization`,
		},
		{
			ID:    "adr-006",
			Type:  "decision",
			Title: "ADR-006: REST over GraphQL for Merchant API",
			Scope: "project",
			Tags:  []string{"rest", "graphql", "api-design", "merchant-api"},
			Content: `# ADR-006: REST over GraphQL for Merchant API

## Status
Accepted

## Context
PayFlow exposes APIs for merchants to create payments, query transaction status, manage refunds, and configure webhooks. We evaluated REST and GraphQL for the merchant-facing API surface. The API design for Payment API v2 (design-001) needed this decision before proceeding.

GraphQL offers flexible querying that would benefit the merchant dashboard, where merchants need varying levels of detail about their transactions. However, the majority of our API traffic is machine-to-machine integration where merchants use our SDKs to process payments programmatically.

## Decision
We will use REST with JSON for all merchant-facing APIs. Specific rationale:

1. Predictable Caching: Payment status (api-002) and transaction listing are our highest-traffic read endpoints. REST's resource-oriented design allows effective HTTP caching with ETags and Cache-Control headers. GraphQL's POST-based queries make HTTP-level caching impossible without a custom caching layer.

2. Idempotency Alignment: Our idempotency key design (ADR-003) maps naturally to REST endpoints. Each resource mutation has a clear HTTP method and URI, making idempotency key scoping straightforward. GraphQL mutations would require custom idempotency scoping per operation.

3. Ecosystem Compatibility: The payment industry standard is REST. Merchants integrating with PayFlow alongside Stripe, Adyen, and Square expect REST APIs. Our webhook delivery system (design-002) also follows REST conventions for event payloads.

4. Security Boundaries: PCI compliance (ADR-005, meeting-002) requires strict control over which data fields are accessible. REST endpoints return fixed response shapes, making it easier to audit data exposure. GraphQL's flexible field selection could inadvertently expose sensitive fields without careful schema design.

5. Documentation: REST APIs have mature tooling (OpenAPI 3.1) for generating interactive documentation, client SDKs, and contract tests. Our API documentation (api-001 through api-005) uses OpenAPI specifications.

We will provide a REST-based Reporting API with pagination, filtering, and field selection (sparse fieldsets per JSON:API) to address the flexible querying needs of merchant dashboards.

## Consequences
- Familiar integration experience for merchants coming from other payment platforms
- Effective HTTP caching for high-traffic status and listing endpoints
- More endpoints to maintain compared to a single GraphQL schema
- Merchants needing custom data views will use the Reporting API or webhooks (design-002)
- OpenAPI specs enable automated SDK generation for Python, Ruby, PHP, Java, and Go

## Related
- design-001: Payment API v2
- ADR-003: Idempotency Keys for Payment APIs
- ADR-005: PCI DSS Tokenization Strategy
- design-002: Webhook Delivery System`,
		},

		// =====================================================================
		// arch-001 through arch-005: Architecture Documents
		// =====================================================================
		{
			ID:    "arch-001",
			Type:  "doc",
			Title: "PayFlow System Overview",
			Scope: "project",
			Tags:  []string{"architecture", "system-overview", "microservices", "payment-platform"},
			Content: `# PayFlow System Overview

## Introduction
PayFlow is a payment processing platform that enables merchants to accept and manage payments across multiple channels (web, mobile, in-store) and payment methods (cards, bank transfers, digital wallets). The platform processes an average of 2.3 million transactions daily with a target availability of 99.99%.

## High-Level Architecture
PayFlow follows a microservices architecture deployed on AWS EKS (Kubernetes). The system is organized into the following bounded contexts:

### Payment Core
The central domain responsible for payment lifecycle management. Uses event sourcing (ADR-001) with the aggregate pattern (pattern-003) to track payment states. The payment processing pipeline (arch-002) orchestrates interactions between internal services and external payment providers.

### Settlement
The settlement engine (arch-003) reconciles authorized and captured payments with provider settlement files, calculates merchant payouts, and manages the funds flow ledger. Settlement runs on a T+1 schedule for domestic transactions and T+2 for international (see arch-005 for multi-currency details).

### Fraud & Risk
The fraud detection system (arch-004) evaluates every transaction in real-time using a combination of rule-based checks, ML models, and velocity counters. It operates as a synchronous step in the payment pipeline with a target latency budget of 50ms.

### Merchant Platform
Handles merchant onboarding (design-003), account management, API key provisioning, and the merchant dashboard. The dashboard uses the REST API (ADR-006) with real-time updates via server-sent events.

## Data Architecture
Primary data store is PostgreSQL with Citus sharding (ADR-002), sharded by merchant_id. Event streams are stored in dedicated event store tables partitioned by time. Redis serves as the caching layer for payment status (design-005) and circuit breaker state (ADR-004, pattern-002).

Apache Kafka serves as the event backbone, carrying domain events between bounded contexts. Event schemas are managed with a schema registry using Avro serialization. The webhook delivery system (design-002) consumes Kafka events to deliver notifications to merchants.

## Infrastructure
- Compute: AWS EKS with Karpenter for autoscaling
- Database: PostgreSQL 16 + Citus on RDS, Redis ElastiCache
- Messaging: Amazon MSK (Managed Kafka)
- Secrets: AWS KMS for encryption keys, HashiCorp Vault for service credentials
- Observability: Datadog for metrics/traces, structured logging to Elasticsearch
- CDN: CloudFront for PayFlow.js SDK distribution (ADR-005)

## Security
All inter-service communication uses mutual TLS. The PCI cardholder data environment is isolated in a dedicated VPC segment (ADR-005). Authentication uses short-lived JWT tokens with scoped permissions (api-005). Secrets rotation is automated on a 90-day cycle.`,
		},
		{
			ID:    "arch-002",
			Type:  "doc",
			Title: "Payment Processing Pipeline Architecture",
			Scope: "project",
			Tags:  []string{"architecture", "payment-pipeline", "orchestration", "providers"},
			Content: `# Payment Processing Pipeline Architecture

## Overview
The payment processing pipeline is the core transaction path in PayFlow, handling the full lifecycle from payment initiation to settlement. The pipeline processes an average of 27,000 transactions per minute at peak, with a p99 latency target of 800ms end-to-end.

## Pipeline Stages

### 1. Request Validation
Incoming payment requests (api-001) are validated for required fields, format correctness, and merchant authorization (api-005). The idempotency check (ADR-003, pattern-001) occurs at this stage, before any side effects.

### 2. Risk Evaluation
The fraud detection system (arch-004) evaluates the transaction synchronously. The risk engine returns an action (approve, decline, review) and a risk score. Transactions flagged for review are held in a pending state and routed to the merchant's review queue.

### 3. Payment Routing
The routing engine selects the optimal payment provider based on:
- Card BIN and network (Visa, Mastercard, Amex, Discover)
- Merchant routing preferences and provider agreements
- Provider health (circuit breaker state per ADR-004, pattern-002)
- Transaction currency and processing region (arch-005)
- Cost optimization (interchange plus pricing)

Routing decisions are recorded as events in the payment aggregate (pattern-003) for auditability.

### 4. Provider Authorization
The selected provider adapter translates the PayFlow payment model into the provider's API format and sends the authorization request. Each provider adapter implements a common ProviderGateway interface:

- Stripe: Full card processing, 3DS authentication, Apple Pay, Google Pay
- Adyen: Card processing, local payment methods (iDEAL, Bancontact, SEPA)
- Braintree: PayPal, Venmo, card processing
- Worldpay: Card processing, optimized for UK/EU transactions

Provider responses are normalized back to PayFlow domain events (PaymentAuthorized, PaymentDeclined) and appended to the event stream (ADR-001).

### 5. Post-Authorization
After successful authorization:
- The payment status projection is updated for real-time queries (design-005)
- Webhook events are enqueued for merchant notification (design-002)
- The settlement engine (arch-003) records the authorization for future settlement
- Fraud detection (arch-004) receives the authorization outcome for model training

### Failure Handling
The pipeline uses the saga orchestration pattern (pattern-004) for operations spanning multiple providers or requiring compensating actions. When a provider call fails, the circuit breaker (pattern-002) evaluates whether to retry, reroute, or fail the transaction. Failed transactions emit PaymentFailed events with detailed error categorization for merchant-facing error messages (api-002).

## Scalability
The pipeline is horizontally scalable. Stateless request processing allows adding instances behind the load balancer. Event sourcing (ADR-001) eliminates write contention since events are append-only. Provider adapters scale independently based on traffic distribution.`,
		},
		{
			ID:    "arch-003",
			Type:  "doc",
			Title: "Settlement Engine Architecture",
			Scope: "project",
			Tags:  []string{"architecture", "settlement", "reconciliation", "ledger", "payouts"},
			Content: `# Settlement Engine Architecture

## Overview
The settlement engine is responsible for reconciling captured payments with provider settlement files, calculating merchant payouts, and maintaining the double-entry funds flow ledger. Settlement is a critical financial process where accuracy takes precedence over speed.

## Settlement Lifecycle

### 1. Capture Tracking
When a payment is captured (either at authorization for auto-capture flows or via a separate capture request), the settlement engine records a pending settlement entry. The entry references the payment aggregate (pattern-003) event stream and the provider authorization ID.

For multi-currency transactions (arch-005), the settlement entry records both the presentment currency (what the cardholder pays) and the settlement currency (what the merchant receives), along with the applicable exchange rate at capture time.

### 2. Provider Reconciliation
Each payment provider delivers settlement files on a regular schedule:
- Stripe: Daily CSV via SFTP, T+1
- Adyen: Daily XML via webhook push and SFTP, T+1
- Braintree: Daily API-based settlement report, T+1
- Worldpay: Daily ISO 8583 format via SFTP, T+1 domestic / T+2 cross-border

The reconciliation process matches provider settlement records against PayFlow's captured transactions using provider transaction IDs. Discrepancies are categorized as:
- Amount mismatches (interchange fee differences, currency conversion variances)
- Missing transactions (captured in PayFlow but not settled by provider)
- Unknown transactions (in provider file but not in PayFlow)

Unresolved discrepancies are escalated to the finance operations team via automated tickets.

### 3. Fee Calculation
The engine calculates PayFlow's processing fees based on the merchant's pricing plan:
- Interchange Plus: Actual interchange + fixed markup
- Flat Rate: Fixed percentage per transaction
- Tiered: Different rates for qualified, mid-qualified, and non-qualified transactions

Refunded transactions (design-004) receive fee credits according to the merchant's agreement. Partial refunds receive proportional fee credits.

### 4. Payout Generation
After reconciliation and fee calculation, the engine generates merchant payout instructions. Payouts are batched by merchant and settlement currency. The payout process:
1. Debits the provider settlement account in the ledger
2. Credits the merchant payout account minus PayFlow fees
3. Credits PayFlow's revenue account for processing fees
4. Submits payout instructions to the banking partner

### Ledger Design
The funds flow ledger uses double-entry accounting. Every financial movement creates a debit and credit entry that must balance. The ledger is append-only (no updates or deletes), consistent with our event sourcing approach (ADR-001). Ledger entries are stored in PostgreSQL (ADR-002) with strong consistency guarantees.

## Operational Concerns
- Settlement runs are idempotent (aligned with pattern-001) to allow safe re-execution
- Reconciliation discrepancy reports are generated daily for finance review
- Payout holds can be applied at the merchant level for risk or compliance reasons
- The Q3 international expansion (meeting-005) will add support for additional settlement currencies`,
		},
		{
			ID:    "arch-004",
			Type:  "doc",
			Title: "Fraud Detection System Architecture",
			Scope: "project",
			Tags:  []string{"architecture", "fraud-detection", "risk", "machine-learning", "security"},
			Content: `# Fraud Detection System Architecture

## Overview
PayFlow's fraud detection system evaluates every payment transaction in real-time to assess fraud risk. It operates as a synchronous step in the payment processing pipeline (arch-002) with a strict 50ms latency budget. The system combines rule-based checks, machine learning models, and velocity-based counters to produce a risk score and recommended action.

## Architecture Components

### Rule Engine
A configurable rule engine evaluates transactions against a set of fraud rules. Rules are organized in priority tiers:

- Block rules: Immediate decline (e.g., sanctioned countries, known fraudulent BINs)
- Flag rules: Force manual review (e.g., transaction amount > merchant's typical average * 5)
- Score rules: Contribute to the composite risk score (e.g., new card + high amount + international)

Merchants can configure custom rules through the merchant dashboard, adding merchant-specific block lists, velocity limits, and amount thresholds. The merchant onboarding flow (design-003) includes default rule configuration based on the merchant's industry vertical.

### ML Risk Model
A gradient-boosted decision tree model (XGBoost) trained on historical transaction data. Features include:
- Transaction attributes: amount, currency, payment method, entry mode
- Card signals: BIN country, issuer, card age, tokenized vs raw (ADR-005)
- Behavioral: merchant transaction patterns, cardholder spending patterns
- Contextual: time of day, device fingerprint, IP geolocation
- Velocity: transaction count and amount in 1min, 1hr, 24hr windows

The model is retrained weekly on a rolling 90-day window. Model performance is monitored via precision/recall metrics and A/B tested against the production model before promotion.

### Velocity Counters
Redis-backed sliding window counters track transaction velocity across multiple dimensions:
- Per card: transactions per minute, per hour, per day
- Per merchant: total volume, decline rate, chargeback rate
- Per IP address: transaction attempts from the same IP
- Per device fingerprint: transaction attempts from the same device

Velocity data is critical for detecting card testing attacks (rapid low-value authorizations) and account takeover patterns. During the November incident (meeting-004), velocity counters helped identify the anomalous traffic pattern.

### 3DS Authentication
For transactions assessed as medium-risk, the system can request 3D Secure authentication (Visa Secure, Mastercard Identity Check) before proceeding with authorization. This shifts liability to the issuing bank while reducing friction for low-risk transactions.

## Integration Points
- Payment pipeline (arch-002): Synchronous risk evaluation at Stage 2
- Event sourcing (ADR-001): Risk decisions recorded as events on the payment aggregate
- Settlement engine (arch-003): Chargeback data feeds back into model training
- Circuit breaker (ADR-004): Provider routing changes are correlated with fraud scoring to avoid false positives during failover

## Monitoring
- Real-time dashboards showing approval rate, decline rate, and review rate by merchant
- Automated alerts for sudden changes in fraud patterns (spike detection)
- Weekly model performance reports comparing predicted risk to actual chargebacks
- Black Friday specific monitoring thresholds (meeting-003) to handle legitimate volume spikes`,
		},
		{
			ID:    "arch-005",
			Type:  "doc",
			Title: "Multi-Currency Support Architecture",
			Scope: "project",
			Tags:  []string{"architecture", "multi-currency", "forex", "international", "settlement"},
			Content: `# Multi-Currency Support Architecture

## Overview
PayFlow supports payment processing in 45 currencies across 30 countries. Multi-currency support touches every layer of the system, from payment acceptance through settlement and merchant payouts. This document describes the architecture for currency handling, foreign exchange, and multi-currency settlement.

## Currency Model

### Presentment Currency
The currency in which the cardholder is charged. Determined by the merchant's payment page configuration or the currency specified in the API request (api-001). All amounts in the payment API are specified in the smallest currency unit (cents for USD/EUR, yen for JPY) to avoid floating-point precision issues.

### Processing Currency
The currency used for the authorization request to the payment provider. Usually matches the presentment currency, but may differ when the provider requires a specific base currency. The payment routing engine (arch-002) selects providers that support the presentment currency natively to avoid unnecessary currency conversions and their associated fees.

### Settlement Currency
The currency in which the merchant receives their payout. Configured during merchant onboarding (design-003). Merchants can configure multiple settlement currencies mapped to different bank accounts.

When the presentment currency differs from the settlement currency, PayFlow applies a foreign exchange conversion. FX rates are sourced from our banking partner and cached for 60-second intervals. The applied rate is locked at capture time and recorded in the settlement entry (arch-003).

## FX Rate Management
- Rate source: ECB reference rates supplemented by banking partner rates for exotic pairs
- Rate refresh: Every 60 seconds during market hours, every 15 minutes outside market hours
- Rate lock: Applied at capture time, valid for 24 hours for delayed captures
- Markup: Configurable per merchant, default 1.5% above mid-market rate
- Rate storage: All applied rates are persisted for reconciliation and dispute resolution

## Multi-Currency Settlement
The settlement engine (arch-003) handles multi-currency reconciliation by maintaining per-currency sub-ledgers. Each provider settlement file specifies amounts in the provider's settlement currency, which may require conversion to the merchant's settlement currency.

Settlement discrepancies due to FX rate differences between capture-time and settlement-time are tracked separately and reported in the daily reconciliation summary.

## Refunds
Refunds (design-004) are processed in the presentment currency. If the FX rate has changed since the original capture, the merchant absorbs the difference. This is standard industry practice and is disclosed during merchant onboarding (design-003).

## International Expansion (Q3)
The Q3 planning meeting (meeting-005) identified the following expansion targets:
- Southeast Asia: SGD, MYF, THB, IDR, PHP processing via Adyen
- Latin America: BRL, MXN, COP processing via a new local provider integration
- Additional European currencies: PLN, CZK, HUF via Worldpay

Each new currency requires BIN table updates, FX rate source validation, settlement currency configuration, and regulatory compliance review.`,
		},

		// =====================================================================
		// design-001 through design-005: Design Documents
		// =====================================================================
		{
			ID:    "design-001",
			Type:  "doc",
			Title: "Design: Payment API v2",
			Scope: "project",
			Tags:  []string{"design", "api", "payment-api", "v2", "merchant-integration"},
			Content: `# Design: Payment API v2

## Overview
Payment API v2 is the next generation merchant-facing API for creating and managing payments. This design addresses limitations in v1: lack of multi-step payment flows (auth-then-capture), inconsistent error responses, and missing support for payment methods beyond cards. The API follows REST conventions (ADR-006) and requires idempotency keys for all mutations (ADR-003).

## Resource Model

### Payment
The central resource. Represents a single payment attempt with the following lifecycle states: requires_action, pending, authorized, captured, partially_refunded, refunded, voided, failed.

### PaymentMethod
A tokenized representation of the customer's payment instrument. Created via client-side tokenization (ADR-005). Supports card, bank_transfer, wallet (Apple Pay, Google Pay), and local payment methods (iDEAL, SEPA Direct Debit).

### Customer
An optional resource for merchants using saved payment methods. Customers can have multiple payment methods attached, enabling one-click checkout flows.

### Refund
A sub-resource of Payment representing a full or partial refund. See design-004 for the refund pipeline design and api-003 for the API specification.

## API Design Principles
1. Consistent resource-oriented URLs: /v2/payments, /v2/payments/{id}, /v2/refunds
2. Idempotency-Key header required for POST, PUT, DELETE (ADR-003, pattern-001)
3. Expand parameters for related resources (e.g., ?expand=payment_method,customer)
4. Pagination via cursor-based pagination with created_at ordering
5. Filtering via query parameters (status, created_after, created_before, amount_min, amount_max)
6. Versioning via URL path (/v2/) with 12-month deprecation policy for v1

## Payment Creation Flow
1. Merchant sends POST /v2/payments with amount, currency, payment_method_id, and optional customer_id
2. Pipeline validates request and checks idempotency (pattern-001)
3. Fraud detection evaluates risk (arch-004)
4. Payment is routed to optimal provider (arch-002)
5. Authorization response is returned synchronously
6. Webhook notification is sent asynchronously (design-002)

For payment methods requiring customer action (3DS, bank redirect), the API returns status requires_action with a redirect URL. The merchant redirects the customer, and PayFlow handles the callback and completes authorization.

## Error Handling
Errors follow RFC 7807 Problem Details format:
- type: URI identifying the error type
- title: Human-readable summary
- detail: Specific error description
- code: Machine-readable error code (e.g., card_declined, insufficient_funds, provider_error)

Provider-specific errors are mapped to PayFlow error codes to provide a consistent merchant experience regardless of which provider processed the transaction.

## Migration from v1
- v1 endpoints remain operational with deprecation headers
- Migration guide maps v1 fields to v2 equivalents
- Webhook event schema (api-004) adds v2 event types alongside v1 for the transition period`,
		},
		{
			ID:    "design-002",
			Type:  "doc",
			Title: "Design: Webhook Delivery System",
			Scope: "project",
			Tags:  []string{"design", "webhooks", "events", "notification", "reliability"},
			Content: `# Design: Webhook Delivery System

## Overview
The webhook delivery system notifies merchants of payment events (authorization, capture, settlement, refund, dispute) in near real-time. Reliable webhook delivery is critical because many merchants use webhooks as their primary mechanism for order fulfillment and reconciliation.

## Architecture

### Event Consumption
The webhook service consumes domain events from Kafka (published by the event sourcing system per ADR-001). Each event is evaluated against merchant webhook subscriptions to determine delivery targets. Merchants configure webhook endpoints and event type filters during onboarding (design-003) or via the API.

### Event Schema
Webhook payloads follow the schema defined in api-004. Each event includes:
- event_id: Unique identifier for the webhook event
- event_type: Namespaced event type (e.g., payment.authorized, payment.captured, refund.completed)
- created_at: ISO 8601 timestamp
- data: The relevant resource in its current state
- api_version: The API version used to render the resource representation

### Delivery Pipeline
1. Event is consumed from Kafka and matched against active webhook subscriptions
2. Payload is rendered using the merchant's configured API version (v1 or v2 per design-001)
3. Delivery attempt: POST to merchant URL with PayFlow-Signature header (HMAC-SHA256)
4. Response handling: HTTP 2xx is success; anything else triggers retry

### Retry Strategy
Failed deliveries are retried with exponential backoff:
- Attempt 1: Immediate
- Attempt 2: 5 minutes
- Attempt 3: 30 minutes
- Attempt 4: 2 hours
- Attempt 5: 8 hours
- Attempt 6: 24 hours (final attempt)

After 6 failed attempts, the event is marked as failed and an email notification is sent to the merchant's technical contact. The merchant can replay failed events from the dashboard.

### Signature Verification
Each webhook includes a PayFlow-Signature header computed as:
HMAC-SHA256(webhook_secret, timestamp + "." + payload_body)

The timestamp prevents replay attacks. Merchants verify the signature using their webhook secret (provisioned during onboarding, design-003) and our SDK helper libraries.

### Ordering Guarantees
Webhook delivery is at-least-once with best-effort ordering. Events for the same payment are delivered in causal order (using the event sequence number from the event store). Cross-payment ordering is not guaranteed.

Merchants must handle duplicate deliveries idempotently. Each event has a unique event_id that merchants should use for deduplication, similar to how our APIs use idempotency keys (ADR-003).

## Monitoring
- Delivery success rate per merchant (target: >99.5%)
- p99 delivery latency (target: <30 seconds for first attempt)
- Failed delivery alerts with merchant-level granularity
- Dashboard showing delivery status per event for merchant self-service troubleshooting`,
		},
		{
			ID:    "design-003",
			Type:  "doc",
			Title: "Design: Merchant Onboarding Flow",
			Scope: "project",
			Tags:  []string{"design", "merchant-onboarding", "kyc", "integration", "compliance"},
			Content: `# Design: Merchant Onboarding Flow

## Overview
Merchant onboarding is the process by which new merchants sign up for PayFlow, complete identity verification (KYC/KYB), configure their payment processing, and integrate with our APIs. The onboarding flow must balance compliance requirements with developer experience to minimize time-to-first-transaction.

## Onboarding Stages

### 1. Account Creation
Merchant creates a PayFlow account with business email. Immediately receives:
- Sandbox API keys for testing
- Access to the merchant dashboard in test mode
- PayFlow.js and mobile SDK credentials for client-side tokenization (ADR-005)

No identity verification is required for sandbox access. This allows developers to start integrating immediately while business verification proceeds in parallel.

### 2. Business Verification (KYC/KYB)
Before processing live transactions, merchants must complete:
- Business identity verification: Legal entity name, registration number, address
- Beneficial ownership: Individuals owning >25% of the business
- Bank account verification: Micro-deposit verification for payout accounts
- Website/app review: Compliance team reviews the merchant's product/service

Verification is processed by our KYC provider (Onfido) with manual review for flagged applications. Target turnaround: 24 hours for standard merchants, 3-5 days for high-risk categories.

### 3. Payment Configuration
After verification, merchants configure:
- Payment methods: Which payment methods to accept (cards, bank transfers, wallets)
- Currency settings: Presentment currencies and settlement currency (arch-005)
- Provider routing: Preferred provider ordering (defaults based on merchant region)
- Fraud rules: Initial rule set based on merchant category (arch-004)
- Webhook endpoints: URLs and event subscriptions (design-002)
- Tokenization settings: Single-use vs multi-use token policies (ADR-005)

### 4. Integration
Merchants integrate using one of:
- Drop-in UI: Pre-built payment form (PayFlow Elements) with minimal code
- Custom integration: PayFlow.js/Mobile SDK for tokenization + REST API (ADR-006, api-001) for server-side processing
- Platform integration: For marketplace models, onboarding connected accounts under a platform merchant

Integration guides are provided with interactive examples and sandbox test card numbers.

### 5. Go-Live Review
Before enabling live processing:
- Integration checklist: Verify error handling, webhook processing, idempotency key usage (ADR-003)
- Test transaction review: Confirm successful test payments in sandbox
- Compliance sign-off: Verify PCI SAQ-A completion (ADR-005)
- Merchant agreement: Digital signature on processing agreement and fee schedule

## Post-Onboarding
- Merchants receive live API keys (rotatable, with key versioning per api-005)
- Automatic enrollment in PayFlow's monitoring and alerting
- Access to transaction analytics and settlement reports (arch-003)
- 30-day onboarding success check by merchant success team

## Q3 Expansion
The international expansion (meeting-005) requires adding country-specific KYC requirements for Southeast Asia and Latin America, including local entity verification and regulatory registration checks.`,
		},
		{
			ID:    "design-004",
			Type:  "doc",
			Title: "Design: Refund Pipeline",
			Scope: "project",
			Tags:  []string{"design", "refunds", "payment-lifecycle", "settlement", "merchant"},
			Content: `# Design: Refund Pipeline

## Overview
The refund pipeline handles full and partial refunds for captured payments. Refunds are one of the most operationally complex flows in PayFlow because they involve coordination between the payment provider, the settlement engine (arch-003), the merchant's balance, and the cardholder's issuing bank. The refund API is documented in api-003.

## Refund Types

### Full Refund
Returns the entire captured amount to the cardholder. The payment transitions to the refunded state in the event stream (ADR-001). The merchant's pending payout is reduced by the refund amount plus any applicable fee reversal.

### Partial Refund
Returns a portion of the captured amount. Multiple partial refunds can be issued against a single payment up to the original capture amount. Each partial refund is tracked as a separate event in the payment aggregate (pattern-003).

### Void
Cancels an authorized but not yet captured payment. Voids release the hold on the cardholder's funds immediately and do not incur processing fees. Voids are only available before the authorization expires (typically 7 days for card payments).

## Refund Processing Flow
1. Merchant submits refund request via API (api-003) with payment_id, amount, and reason
2. Idempotency check (ADR-003, pattern-001) prevents duplicate refund processing
3. Validation: verify payment is in captured or partially_refunded state, refund amount does not exceed remaining capturable amount
4. The saga orchestrator (pattern-004) initiates the refund:
   a. Submit refund to the original payment provider via the same provider adapter used for the initial authorization
   b. Record RefundInitiated event on the payment aggregate
   c. Wait for provider confirmation (synchronous for most providers)
   d. Record RefundCompleted or RefundFailed event
5. Settlement engine (arch-003) adjusts the merchant's settlement:
   a. If the original transaction has not yet settled: reduce the pending settlement amount
   b. If already settled: create a debit entry in the next settlement batch
   c. Fee reversal: credit the processing fee proportional to the refund amount
6. Webhook notification (design-002) sent with refund.completed or refund.failed event
7. Payment status (design-005) updated to reflect the refund

## Multi-Currency Refunds
Refunds are processed in the original presentment currency (arch-005). If the merchant's settlement currency differs, the FX rate at refund time may differ from the original capture rate, resulting in an FX gain or loss for the merchant. This is standard industry practice and is disclosed in the merchant agreement (design-003).

## Edge Cases
- Refund after chargeback: Blocked to prevent double-refund
- Refund after provider settlement window: Requires manual processing by operations
- Provider timeout during refund: Saga compensation queues a retry with exponential backoff
- Partial refund reducing amount below minimum: Requires full refund instead

## Monitoring
- Refund rate per merchant (elevated rates trigger fraud review via arch-004)
- Refund processing latency p99 target: 5 seconds
- Failed refund alerts with automated retry status tracking`,
		},
		{
			ID:    "design-005",
			Type:  "doc",
			Title: "Design: Payment Status Tracking",
			Scope: "project",
			Tags:  []string{"design", "payment-status", "cqrs", "real-time", "merchant-experience"},
			Content: `# Design: Payment Status Tracking

## Overview
Payment status tracking provides merchants with real-time visibility into their payment lifecycle. Because PayFlow uses event sourcing (ADR-001) with CQRS, the read model for payment status is a projection that is eventually consistent with the write model. This design describes how we maintain the status projection, handle consistency, and provide real-time updates.

## Status Model
A payment progresses through the following states:
- requires_action: Awaiting customer action (3DS, bank redirect)
- pending: Processing initiated, awaiting provider response
- authorized: Successfully authorized, funds held on cardholder's account
- captured: Funds captured, pending settlement
- partially_refunded: One or more partial refunds processed
- refunded: Fully refunded
- voided: Authorization cancelled before capture
- failed: Authorization or capture failed
- disputed: Cardholder has filed a chargeback

Each state transition is recorded as an event (ADR-001) and projected to the status read model.

## Read Model Architecture

### PostgreSQL Projection
The primary status projection is a denormalized table in PostgreSQL (ADR-002) that stores the current state of each payment along with key attributes (amount, currency, merchant_id, created_at, provider, last_event_at). This table serves the GET /v2/payments/{id} endpoint (api-002) and the payment listing endpoint.

The projection is updated by an event consumer that subscribes to the payment event stream via Kafka. Updates are idempotent using the event sequence number to prevent out-of-order processing.

### Redis Cache
A Redis cache (ADR-002) stores the latest payment status for high-frequency lookups. The cache is populated by the same event consumer and has a 30-second TTL. Cache misses fall through to PostgreSQL.

The cache enables sub-5ms p99 latency for the status endpoint (api-002), which merchants poll frequently after creating a payment.

## Consistency Guarantees
The status projection is eventually consistent with the event store. Under normal conditions, the projection lag is <500ms. During high-traffic periods (meeting-003, Black Friday), lag may increase to 2-3 seconds.

To handle the consistency window:
1. The create payment response includes the initial status directly from the event (no projection dependency)
2. Subsequent GET requests may serve slightly stale data from the projection
3. The response includes a last_event_at timestamp so merchants can detect staleness
4. Merchants requiring strong consistency can use the event stream directly via webhooks (design-002)

## Real-Time Updates
In addition to polling the status endpoint, merchants can subscribe to real-time updates via:
- Webhooks (design-002): Push-based notification for all payment events
- Server-Sent Events (SSE): Streaming updates for the merchant dashboard
- The PayFlow.js SDK includes a status polling helper with configurable intervals and automatic backoff

## Monitoring
- Projection lag: p99 target <1 second, alert at >5 seconds
- Cache hit rate: Target >95%, alert at <85%
- Status endpoint latency: p99 target <10ms (cache hit), <50ms (cache miss)
- Event consumer health: Kafka consumer group lag monitoring with auto-scaling`,
		},

		// =====================================================================
		// meeting-001 through meeting-005: Meeting Notes
		// =====================================================================
		{
			ID:    "meeting-001",
			Type:  "context",
			Title: "Meeting Notes: Q1 Kickoff - Payment Platform",
			Scope: "project",
			Tags:  []string{"meeting", "planning", "q1", "roadmap", "payment-platform"},
			Content: `# Meeting Notes: Q1 Kickoff - Payment Platform

**Date:** January 8, 2025
**Attendees:** Sarah (VP Eng), Marcus (Tech Lead), Priya (Staff Eng), David (Product), Lisa (Security), James (SRE)

## Agenda
Review Q1 objectives for the PayFlow payment platform, align on technical priorities, and identify risks.

## Key Decisions

### 1. Payment API v2 is Q1 Priority
David presented merchant feedback on API v1 pain points: no auth-then-capture flow, inconsistent error codes, and lack of saved payment methods. The team agreed that Payment API v2 (design-001) is the top priority for Q1.

Marcus proposed using this as the opportunity to finalize the event sourcing migration (ADR-001). The API v2 will be built natively on the event-sourced payment model rather than retrofitting v1. Priya will lead the aggregate pattern implementation (pattern-003).

### 2. Database Migration Decision
Marcus presented the PostgreSQL vs DynamoDB analysis (ADR-002). After discussion, the team approved PostgreSQL with Citus. Key factor: the settlement engine (arch-003) queries are too complex for DynamoDB's query model. James confirmed the SRE team can manage PostgreSQL at scale with their existing expertise.

### 3. Idempotency Implementation
Priya raised the duplicate payment issue from December (3 merchants reported duplicate charges due to network retry). The team agreed that idempotency keys (ADR-003) must be part of API v2 from day one. Priya will document the implementation pattern (pattern-001).

### 4. PCI Compliance Timeline
Lisa reported that our PCI DSS Level 1 audit is scheduled for March. The tokenization strategy (ADR-005) must be finalized and implemented before the audit. Lisa will schedule a dedicated PCI compliance review (meeting-002) for the following week.

## Action Items
- [ ] Marcus: Finalize ADR-001 (event sourcing) and ADR-002 (PostgreSQL) - Due Jan 15
- [ ] Priya: Write pattern-001 (idempotent processing) and pattern-003 (aggregate) - Due Jan 22
- [ ] David: Draft API v2 specification (design-001, api-001 through api-003) - Due Jan 29
- [ ] Lisa: Schedule PCI compliance review, prepare ADR-005 draft - Due Jan 12
- [ ] James: Capacity planning for PostgreSQL cluster, Redis cluster sizing - Due Jan 22
- [ ] Sarah: Approve headcount for two additional backend engineers

## Risks
- PCI audit timeline is tight - tokenization must be production-ready by end of February
- Event sourcing migration may impact Black Friday readiness if not completed by Q2
- Merchant migration from API v1 to v2 needs dedicated developer relations support

## Next Steps
- Weekly sync on Tuesdays at 10am
- Design reviews for API v2 and webhook system (design-002) in sprint 2
- PCI compliance deep-dive next week (meeting-002)`,
		},
		{
			ID:    "meeting-002",
			Type:  "context",
			Title: "Meeting Notes: PCI Compliance Review",
			Scope: "project",
			Tags:  []string{"meeting", "pci-dss", "compliance", "security", "audit"},
			Content: `# Meeting Notes: PCI Compliance Review

**Date:** January 15, 2025
**Attendees:** Lisa (Security), Marcus (Tech Lead), Priya (Staff Eng), James (SRE), external: Chen (PCI QSA)

## Agenda
Review current PCI DSS compliance posture, finalize tokenization strategy (ADR-005), and prepare for the March Level 1 audit.

## Current State Assessment
Chen (our Qualified Security Assessor) reviewed our current architecture and identified the following:

### In Scope (CDE)
- Payment processing pipeline (arch-002) - handles detokenized card data during provider calls
- Token vault service - stores encrypted PANs
- Provider adapter services - transmit card data to Stripe, Adyen, etc.

### Out of Scope (with tokenization)
- Merchant API gateway - only sees tokens, never raw card data
- Settlement engine (arch-003) - works with transaction IDs, not card data
- Fraud detection (arch-004) - uses BIN data and tokens, not full PANs
- Merchant dashboard and webhook system (design-002)

Chen confirmed that our proposed tokenization strategy (ADR-005) would significantly reduce the CDE scope. The client-side tokenization approach means our API servers never touch raw cardholder data.

## Key Discussions

### Network Tokenization
Chen strongly recommended implementing network tokens (Visa Token Service, Mastercard MDES) as described in ADR-005. Beyond reducing PCI scope, network tokens improve authorization rates by 2-3% because issuers trust network-level tokens more than merchant-level tokens.

Marcus raised a concern about token lifecycle complexity. Network tokens can be updated by the network when a card is reissued, requiring webhook handling for token updates. Priya will add this to the token vault design.

### Key Management
Lisa presented the KMS architecture: per-merchant encryption keys stored in AWS KMS with automatic rotation every 365 days. Chen approved but recommended adding key usage logging for audit trail.

James confirmed that the PCI-scoped network segment is isolated with security groups allowing only the token vault and provider adapters to communicate with card networks. All other services access payment data only through tokens.

### Logging and Monitoring
Chen flagged that our current logging configuration in the payment pipeline (arch-002) includes request/response bodies which could inadvertently log card data. Action: implement structured logging with PAN detection and masking before the audit.

Lisa will add log scanning rules to our SIEM to detect any accidental cardholder data exposure.

## Audit Preparation Timeline
- January 31: Complete tokenization implementation (ADR-005)
- February 7: Complete logging remediation (PAN masking)
- February 14: Internal security assessment (pre-audit dry run)
- February 28: Penetration test by third-party firm
- March 10-14: On-site PCI DSS Level 1 audit with Chen's team

## Action Items
- [ ] Priya: Implement network token lifecycle handling in token vault
- [ ] Marcus: Implement PAN detection/masking in structured logging
- [ ] James: Document network segmentation and firewall rules for CDE
- [ ] Lisa: Prepare evidence binder (policies, procedures, architecture diagrams)
- [ ] Lisa: Schedule penetration test with NCC Group for February 28`,
		},
		{
			ID:    "meeting-003",
			Type:  "context",
			Title: "Meeting Notes: Performance Review - Black Friday Prep",
			Scope: "project",
			Tags:  []string{"meeting", "performance", "black-friday", "scaling", "load-testing"},
			Content: `# Meeting Notes: Performance Review - Black Friday Prep

**Date:** September 12, 2025
**Attendees:** James (SRE), Marcus (Tech Lead), Priya (Staff Eng), Amir (Performance Eng), Sarah (VP Eng)

## Agenda
Review system readiness for Black Friday / Cyber Monday (BFCM) traffic. Last year's BFCM peak was 45,000 transactions per minute. Projected peak this year: 75,000 TPM based on merchant growth.

## Load Test Results

### Payment Pipeline (arch-002)
Amir presented load test results at 100,000 TPM (1.33x projected peak):
- Authorization latency p50: 180ms, p95: 420ms, p99: 780ms (target: <800ms p99) - PASS
- Throughput: Sustained 100K TPM for 2 hours without degradation - PASS
- Error rate: 0.02% (all provider-side timeouts) - PASS
- CPU utilization: 65% average across 24 pods - headroom acceptable

### PostgreSQL (ADR-002)
- Write throughput: 52,000 events/second sustained - PASS
- Read latency (status endpoint via cache): p99 4ms - PASS
- Read latency (cache miss): p99 38ms - PASS
- Connection pool utilization: 78% at peak - needs monitoring but acceptable
- Citus shard rebalance: Completed, merchant distribution is even

James flagged that the Citus coordinator node is the bottleneck for cross-shard queries used by settlement (arch-003). Recommendation: defer non-critical settlement queries to read replicas during BFCM.

### Redis
- Cache hit rate for payment status (design-005): 97.2% - PASS
- Circuit breaker state reads (pattern-002): sub-millisecond - PASS
- Velocity counter updates (arch-004): 180K writes/second with cluster - PASS
- Memory: 42% utilization, projected 58% at 75K TPM - acceptable

### Kafka
- Event throughput: 200K events/second sustained - PASS
- Consumer lag for webhook delivery (design-002): <2 seconds at peak - PASS
- Consumer lag for status projection (design-005): <1 second at peak - PASS

## Identified Risks

### 1. Provider Rate Limits
Stripe's authorization rate limit is 100 requests/second per account. At projected peak, we will need to distribute across multiple Stripe accounts or negotiate a limit increase. Marcus will contact Stripe's enterprise team.

### 2. Circuit Breaker Sensitivity
Amir discovered that during sustained high load, transient provider latency spikes trigger the circuit breaker (ADR-004, pattern-002) too aggressively. The 5-consecutive-failures threshold is too low at high volume. Recommendation: switch to percentage-based failure detection (>50% failure rate in 60-second window) for BFCM.

### 3. Webhook Delivery Backlog
At 100K TPM, webhook delivery (design-002) develops a 45-second backlog. Merchants relying on webhooks for order fulfillment will see delayed notifications. Recommendation: scale webhook delivery workers to 3x current count for BFCM.

## Action Items
- [ ] Marcus: Negotiate Stripe rate limit increase for BFCM
- [ ] Priya: Adjust circuit breaker thresholds per risk #2
- [ ] James: Scale webhook workers, add auto-scaling triggers
- [ ] James: Configure settlement query routing to read replicas during BFCM
- [ ] Amir: Run follow-up load test at 120K TPM after optimizations
- [ ] Sarah: Approve budget for additional infrastructure during Nov 20-Dec 5`,
		},
		{
			ID:    "meeting-004",
			Type:  "context",
			Title: "Meeting Notes: Incident Review - November Payment Failures",
			Scope: "project",
			Tags:  []string{"meeting", "incident", "postmortem", "payment-failures", "stripe"},
			Content: `# Meeting Notes: Incident Review - November Payment Failures

**Date:** November 18, 2024
**Attendees:** James (SRE), Marcus (Tech Lead), Priya (Staff Eng), Sarah (VP Eng), Ops on-call: Kenji

**Incident ID:** INC-2024-0847
**Duration:** 45 minutes (14:23 - 15:08 UTC)
**Severity:** SEV-1
**Impact:** 12,400 failed payment authorizations, affecting 340 merchants

## Timeline
- 14:23 UTC: Datadog alert fires for elevated 5xx rate on POST /v1/payments
- 14:25 UTC: Kenji acknowledges alert, begins investigation
- 14:28 UTC: Root cause identified - Stripe API returning HTTP 503 for all authorization requests
- 14:30 UTC: Kenji confirms Stripe status page shows "Elevated Error Rates" for Payment Intents API
- 14:32 UTC: Kenji escalates to Marcus. Key issue: PayFlow does not have automatic failover when Stripe degrades. All Stripe-routed transactions are failing with 30-second timeouts.
- 14:38 UTC: Marcus begins manual rerouting of Stripe traffic to Adyen. This requires config changes and deployment.
- 14:52 UTC: Manual rerouting deployed. New transactions begin processing via Adyen.
- 14:55 UTC: Queued Stripe transactions (captures on existing Stripe authorizations) cannot be rerouted - these must wait for Stripe recovery.
- 15:08 UTC: Stripe reports recovery. Queued captures processed successfully within 10 minutes.

## Root Cause
Stripe experienced a 45-minute degradation of their Payment Intents API. PayFlow's payment pipeline (arch-002) had no circuit breaker mechanism at the time. Requests to Stripe continued accumulating with 30-second timeouts, exhausting the connection pool and causing cascading latency across all providers.

## Impact Analysis
- 12,400 authorization requests failed during the incident
- 8,200 of these could have been routed to alternative providers (Adyen, Braintree)
- 4,200 were captures on existing Stripe authorizations - no rerouting possible
- Estimated merchant revenue impact: $2.1M in delayed or lost sales
- 47 merchant support tickets filed

## Contributing Factors
1. No circuit breaker pattern for provider integrations (now addressed by ADR-004)
2. 30-second timeout too long for payment provider calls (reduced to 5 seconds)
3. Manual rerouting required config change + deployment (24 minutes to execute)
4. Fraud detection system (arch-004) velocity counters flagged increased declines but alerts were informational-only, not actionable
5. Monitoring did not differentiate provider-specific error rates

## Action Items (Completed)
- [x] Implement circuit breaker pattern for all providers (ADR-004, pattern-002)
- [x] Reduce provider timeout from 30s to 5s
- [x] Implement automatic provider failover in payment routing (arch-002)
- [x] Add per-provider error rate dashboards and alerts
- [x] Document incident response runbook for provider failures

## Action Items (Pending)
- [ ] Load test provider failover under realistic traffic (meeting-003)
- [ ] Implement queuing for non-routable transactions (captures, refunds) during outages
- [ ] Add provider health endpoint to merchant dashboard

## Lessons Learned
This incident drove the decision to implement the circuit breaker pattern (ADR-004, pattern-002) as a core infrastructure component. The manual failover process demonstrated that automated routing decisions are essential at our transaction volume. The Black Friday prep (meeting-003) will specifically validate the circuit breaker behavior under load.`,
		},
		{
			ID:    "meeting-005",
			Type:  "context",
			Title: "Meeting Notes: Q3 Planning - International Expansion",
			Scope: "project",
			Tags:  []string{"meeting", "planning", "q3", "international", "expansion", "multi-currency"},
			Content: `# Meeting Notes: Q3 Planning - International Expansion

**Date:** June 5, 2025
**Attendees:** Sarah (VP Eng), Marcus (Tech Lead), David (Product), Priya (Staff Eng), Lisa (Security), Raj (Partnerships)

## Agenda
Plan the Q3 international expansion initiative. PayFlow currently processes in 12 currencies across 8 countries. Goal: expand to 45 currencies across 30 countries by end of Q3.

## Market Priorities

### Southeast Asia (Priority 1)
Raj presented partnership agreements with local acquirers:
- Singapore (SGD): Live via Adyen, need local payment methods (PayNow, GrabPay)
- Malaysia (MYR): New integration with local acquirer iPay88 required
- Thailand (THB): PromptPay integration via Adyen
- Indonesia (IDR): GoPay, OVO, and Dana wallet integrations required
- Philippines (PHP): GCash and Maya integration via new local partner

David noted that Southeast Asian markets have high mobile wallet adoption (60-80%) compared to card payments. Our current architecture supports wallets (design-001) but the provider adapter framework (arch-002) needs extension for local wallet providers.

### Latin America (Priority 2)
- Brazil (BRL): Pix (instant payment) and Boleto integrations via new provider EBANX
- Mexico (MXN): OXXO (cash voucher) and SPEI (bank transfer) via EBANX
- Colombia (COP): PSE bank transfer integration

Latin American markets require handling of local tax IDs (CPF in Brazil, RFC in Mexico) in payment requests. The Payment API v2 (design-001) needs to support local_fields in the payment creation request.

### Additional European Currencies
- Poland (PLN), Czech Republic (CZK), Hungary (HUF) via Worldpay
- These currencies are straightforward additions to our existing multi-currency support (arch-005)

## Technical Requirements

### Multi-Currency Architecture
Marcus confirmed that the multi-currency support architecture (arch-005) was designed for this expansion. Key work items:
1. Add BIN tables for new card networks in target markets
2. Integrate FX rate sources for new currency pairs
3. Configure settlement currencies and payout rails for each market
4. Update the settlement engine (arch-003) for new provider reconciliation formats

### Provider Integration Framework
Priya estimated 6-8 weeks per new provider integration. Each integration requires:
- Provider adapter implementation following the ProviderGateway interface (arch-002)
- Circuit breaker configuration (ADR-004, pattern-002)
- Payment method mapping and routing rules
- Settlement file parser for reconciliation (arch-003)
- Sandbox testing environment setup

### Compliance
Lisa identified country-specific requirements:
- Indonesia: Bank Indonesia licensing requirements
- Brazil: Central Bank of Brazil registration for foreign payment processors
- Data residency: Singapore requires payment data to be processable locally
- Lisa will assess whether we need regional deployment or data replication

### Merchant Onboarding
The merchant onboarding flow (design-003) needs country-specific KYC requirements:
- Local entity verification documents
- Country-specific regulatory disclosures
- Multi-currency payout configuration (arch-005)

## Timeline
- July: Provider integration kick-off (EBANX, iPay88)
- August: Multi-currency and settlement configuration, compliance approvals
- September: Merchant beta launch in priority markets
- October: General availability

## Risks
- Regulatory approval timelines are unpredictable (especially Indonesia, Brazil)
- New provider integrations may introduce reliability risks during BFCM if not thoroughly tested
- FX rate volatility in emerging market currencies requires careful margin management`,
		},

		// =====================================================================
		// api-001 through api-005: API Documentation
		// =====================================================================
		{
			ID:    "api-001",
			Type:  "doc",
			Title: "API: Create Payment Endpoint",
			Scope: "project",
			Tags:  []string{"api", "payment", "create", "endpoint", "rest"},
			Content: `# API: Create Payment Endpoint

## POST /v2/payments

Creates a new payment. This is the primary entry point for processing a payment through PayFlow. The endpoint follows the design specified in design-001 and implements the payment processing pipeline described in arch-002.

## Authentication
Bearer token authentication (api-005). Requires the payments:write scope.

## Headers
| Header | Required | Description |
|--------|----------|-------------|
| Authorization | Yes | Bearer {api_key} |
| Idempotency-Key | Yes | Unique key for idempotent processing (ADR-003, pattern-001) |
| Content-Type | Yes | application/json |
| PayFlow-Version | No | API version (default: 2025-01-15) |

## Request Body
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| amount | integer | Yes | Amount in smallest currency unit (e.g., cents) |
| currency | string | Yes | ISO 4217 currency code (see arch-005 for supported currencies) |
| payment_method_id | string | Yes* | Token from client-side tokenization (ADR-005) |
| payment_method | object | Yes* | Inline payment method (alternative to payment_method_id) |
| customer_id | string | No | Customer ID for saved payment methods |
| capture | boolean | No | Auto-capture on authorization (default: true) |
| description | string | No | Payment description (appears on cardholder statement) |
| metadata | object | No | Key-value pairs for merchant use (max 50 keys) |
| shipping | object | No | Shipping address for AVS and fraud detection (arch-004) |
| return_url | string | Conditional | Required for 3DS and redirect-based payment methods |
| local_fields | object | No | Market-specific fields (e.g., CPF for Brazil per meeting-005) |

*Either payment_method_id or payment_method must be provided.

## Response (201 Created)
Returns the Payment resource:
{
  "id": "pay_1a2b3c4d5e6f",
  "object": "payment",
  "status": "authorized",
  "amount": 5000,
  "currency": "usd",
  "payment_method": { "id": "pm_abc123", "type": "card", "last4": "4242", "brand": "visa" },
  "captured": false,
  "created_at": "2025-01-15T10:30:00Z",
  "metadata": {},
  "idempotency_key": "merchant-order-12345"
}

## Status Values
See design-005 for the complete payment status model:
- requires_action: Customer action needed (3DS, redirect)
- pending: Processing in progress
- authorized: Funds held, ready for capture
- captured: Funds captured
- failed: Authorization failed

## Error Responses
Errors follow RFC 7807 format (design-001):
- 400: Invalid request (missing fields, invalid currency)
- 401: Invalid or missing API key (api-005)
- 402: Payment declined (insufficient funds, card declined)
- 409: Idempotency key in use for in-flight request (ADR-003)
- 422: Idempotency key reused with different request body (ADR-003)
- 429: Rate limit exceeded
- 500: Internal error (triggers circuit breaker evaluation per ADR-004)

## Rate Limits
- Default: 100 requests/second per merchant
- Enterprise: Configurable up to 1,000 requests/second
- Rate limit headers: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset

## Webhooks
On successful authorization, a payment.authorized webhook event (api-004) is sent to configured endpoints (design-002). If auto-capture is enabled, a payment.captured event follows.`,
		},
		{
			ID:    "api-002",
			Type:  "doc",
			Title: "API: Get Payment Status",
			Scope: "project",
			Tags:  []string{"api", "payment-status", "endpoint", "rest", "query"},
			Content: `# API: Get Payment Status

## GET /v2/payments/{payment_id}

Retrieves the current status and details of a payment. This is the most frequently called endpoint in the PayFlow API, used by merchants to check authorization results, track captures, and monitor refund progress. The status data is served from the read model projection described in design-005.

## Authentication
Bearer token authentication (api-005). Requires the payments:read scope.

## Path Parameters
| Parameter | Type | Description |
|-----------|------|-------------|
| payment_id | string | The payment identifier (e.g., pay_1a2b3c4d5e6f) |

## Query Parameters
| Parameter | Type | Description |
|-----------|------|-------------|
| expand | string | Comma-separated list of related resources to include: payment_method, customer, refunds, events |

## Response (200 OK)
Returns the Payment resource with current status:
{
  "id": "pay_1a2b3c4d5e6f",
  "object": "payment",
  "status": "captured",
  "amount": 5000,
  "amount_captured": 5000,
  "amount_refunded": 0,
  "currency": "usd",
  "payment_method": { "id": "pm_abc123", "type": "card", "last4": "4242", "brand": "visa" },
  "captured": true,
  "captured_at": "2025-01-15T10:30:05Z",
  "created_at": "2025-01-15T10:30:00Z",
  "metadata": { "order_id": "ORD-789" },
  "last_event_at": "2025-01-15T10:30:05Z",
  "provider": "stripe"
}

## Consistency Model
The payment status endpoint serves data from the CQRS read model (design-005). Under normal conditions, the read model is updated within 500ms of the write. During peak traffic (meeting-003), lag may increase to 2-3 seconds.

The last_event_at field indicates the timestamp of the most recent event projected for this payment. Merchants can compare this against the webhook delivery timestamp to detect stale reads.

For use cases requiring strong consistency (e.g., immediately after creating a payment), the create payment response (api-001) returns the authoritative status directly from the event store, bypassing the projection.

## Caching
The endpoint supports HTTP caching:
- ETag header based on the event sequence number
- Cache-Control: private, max-age=5 (5-second client cache)
- Conditional requests with If-None-Match return 304 Not Modified

Server-side, the status is cached in Redis (ADR-002, design-005) with a 30-second TTL. Cache hit rate is >95% under normal conditions.

## Expand Parameter
The expand parameter allows including related resources in a single request:
- payment_method: Full payment method details (masked card number, bank name)
- customer: Customer details if the payment was associated with a customer
- refunds: List of refunds against this payment (see api-003, design-004)
- events: Chronological list of payment events (from the event store per ADR-001)

Expanding events returns the full audit trail for the payment, useful for debugging and dispute resolution.

## Error Responses
- 401: Invalid or missing API key (api-005)
- 404: Payment not found (or belongs to a different merchant)
- 429: Rate limit exceeded
- 500: Internal error

## Rate Limits
- Default: 200 requests/second per merchant
- The status endpoint has higher rate limits than mutation endpoints due to frequent polling patterns
- Merchants are encouraged to use webhooks (design-002) instead of polling for production integrations`,
		},
		{
			ID:    "api-003",
			Type:  "doc",
			Title: "API: Process Refund",
			Scope: "project",
			Tags:  []string{"api", "refund", "endpoint", "rest", "payment-lifecycle"},
			Content: `# API: Process Refund

## POST /v2/payments/{payment_id}/refunds

Creates a refund for a previously captured payment. Supports full and partial refunds. The refund processing flow is described in design-004. This endpoint follows the idempotency pattern (ADR-003, pattern-001).

## Authentication
Bearer token authentication (api-005). Requires the refunds:write scope.

## Headers
| Header | Required | Description |
|--------|----------|-------------|
| Authorization | Yes | Bearer {api_key} |
| Idempotency-Key | Yes | Unique key for idempotent processing (ADR-003) |
| Content-Type | Yes | application/json |

## Path Parameters
| Parameter | Type | Description |
|-----------|------|-------------|
| payment_id | string | The payment to refund (e.g., pay_1a2b3c4d5e6f) |

## Request Body
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| amount | integer | No | Refund amount in smallest currency unit. Omit for full refund. |
| reason | string | No | Reason code: duplicate, fraudulent, requested_by_customer, other |
| metadata | object | No | Key-value pairs for merchant use |

## Response (201 Created)
Returns the Refund resource:
{
  "id": "ref_7g8h9i0j1k2l",
  "object": "refund",
  "payment_id": "pay_1a2b3c4d5e6f",
  "status": "completed",
  "amount": 2500,
  "currency": "usd",
  "reason": "requested_by_customer",
  "created_at": "2025-01-20T14:00:00Z",
  "metadata": {}
}

## Refund Status Values
- pending: Refund submitted to provider, awaiting confirmation
- completed: Refund confirmed by provider
- failed: Refund rejected by provider (e.g., original transaction too old)

## Validation Rules
The refund pipeline (design-004) enforces the following:
1. Payment must be in captured or partially_refunded status
2. Refund amount must not exceed remaining refundable amount (original amount minus previous refunds)
3. Minimum refund amount: 50 cents (or equivalent in presentment currency per arch-005)
4. Refunds cannot be processed on disputed payments (chargeback in progress)
5. Provider settlement window: Some providers reject refunds after 180 days

## Multi-Currency
Refunds are processed in the original presentment currency (arch-005, design-004). The refund amount is specified in the same currency unit as the original payment. FX rate differences between capture and refund time are absorbed by the merchant.

## Settlement Impact
Per the settlement engine (arch-003) and design-004:
- Pre-settlement refunds reduce the merchant's pending settlement amount
- Post-settlement refunds create a debit entry in the next settlement batch
- Fee credits are applied proportionally to the refund amount

## Webhooks
On refund completion, a refund.completed webhook event (api-004, design-002) is sent to configured endpoints. Failed refunds trigger a refund.failed event.

## Error Responses
- 400: Invalid request (negative amount, invalid reason code)
- 401: Invalid or missing API key (api-005)
- 404: Payment not found
- 409: Idempotency key in use for in-flight request
- 422: Refund validation failed (exceeds refundable amount, payment not captured, chargeback active)
- 429: Rate limit exceeded

## Listing Refunds
## GET /v2/payments/{payment_id}/refunds
Returns a paginated list of refunds for a payment. Supports cursor-based pagination and filtering by status and created_at range. Requires payments:read scope.`,
		},
		{
			ID:    "api-004",
			Type:  "doc",
			Title: "API: Webhook Event Schema",
			Scope: "project",
			Tags:  []string{"api", "webhooks", "events", "schema", "notification"},
			Content: `# API: Webhook Event Schema

## Overview
PayFlow sends webhook events to notify merchants of payment lifecycle changes. This document defines the event schema and available event types. The webhook delivery mechanism is described in design-002.

## Event Envelope
All webhook events share a common envelope structure:
{
  "id": "evt_3m4n5o6p7q8r",
  "object": "event",
  "type": "payment.authorized",
  "api_version": "2025-01-15",
  "created_at": "2025-01-15T10:30:01Z",
  "data": {
    "object": { ... }
  },
  "request": {
    "id": "req_abc123",
    "idempotency_key": "merchant-order-12345"
  }
}

## Event Fields
| Field | Type | Description |
|-------|------|-------------|
| id | string | Unique event identifier. Use for deduplication (see ADR-003 for idempotency context) |
| object | string | Always "event" |
| type | string | Event type (see below) |
| api_version | string | API version used to render the data object |
| created_at | string | ISO 8601 timestamp |
| data.object | object | The resource in its current state (Payment, Refund, Dispute) |
| request.id | string | The API request that triggered this event (null for provider-initiated events) |
| request.idempotency_key | string | The idempotency key from the original request |

## Payment Events
Events following the payment lifecycle defined in design-005:

| Event Type | Trigger | Data Object |
|-----------|---------|-------------|
| payment.created | Payment initiated | Payment |
| payment.requires_action | Customer action needed (3DS) | Payment |
| payment.authorized | Authorization successful | Payment |
| payment.authorization_failed | Authorization declined | Payment |
| payment.captured | Capture successful | Payment |
| payment.capture_failed | Capture failed | Payment |
| payment.voided | Authorization voided | Payment |
| payment.settled | Settlement confirmed (arch-003) | Payment |

## Refund Events
Events from the refund pipeline (design-004):

| Event Type | Trigger | Data Object |
|-----------|---------|-------------|
| refund.created | Refund initiated | Refund |
| refund.completed | Refund confirmed by provider | Refund |
| refund.failed | Refund rejected by provider | Refund |

## Dispute Events
| Event Type | Trigger | Data Object |
|-----------|---------|-------------|
| dispute.created | Chargeback filed by cardholder | Dispute |
| dispute.updated | Dispute status changed | Dispute |
| dispute.won | Dispute resolved in merchant's favor | Dispute |
| dispute.lost | Dispute resolved in cardholder's favor | Dispute |

## Event Sourcing Alignment
Webhook events are derived from domain events in the event sourcing system (ADR-001). Each domain event (PaymentAuthorized, PaymentCaptured, etc.) is mapped to the corresponding webhook event type. The event consumer in the webhook delivery system (design-002) performs this mapping and renders the current resource state using the merchant's configured API version.

Note that webhook events represent the current state of the resource, not the delta. The data.object contains the full Payment or Refund resource as it would be returned by the corresponding GET endpoint (api-002 for payments, api-003 for refunds).

## Versioning
The api_version field determines how the data object is rendered. During the v1 to v2 migration (design-001), merchants can receive events in either version. The event type namespace is consistent across versions; only the data object schema differs.

## Signature Verification
See design-002 for the HMAC-SHA256 signature scheme. Merchants must verify webhook signatures to prevent spoofing. Our SDK libraries include helper methods for signature verification.

## Best Practices
1. Always verify the webhook signature before processing
2. Respond with HTTP 2xx within 5 seconds (offload processing to a background queue)
3. Use the event id for idempotent processing to handle duplicate deliveries
4. Do not rely on webhook ordering across different payments
5. Store raw webhook payloads for debugging and dispute resolution`,
		},
		{
			ID:    "api-005",
			Type:  "doc",
			Title: "API: Authentication and Authorization",
			Scope: "project",
			Tags:  []string{"api", "authentication", "authorization", "security", "api-keys"},
			Content: `# API: Authentication and Authorization

## Overview
PayFlow uses API key authentication with scope-based authorization for all merchant-facing APIs. This document covers authentication mechanisms, key management, and authorization scopes. The authentication system integrates with the PCI tokenization strategy (ADR-005) to ensure only authorized services access sensitive payment data.

## API Key Types

### Secret Keys
Server-side keys used for API requests. Format: sk_live_xxx or sk_test_xxx.
- Used in the Authorization header: Bearer sk_live_xxx
- Must never be exposed in client-side code or version control
- Scoped to specific permissions (see Authorization Scopes below)

### Publishable Keys
Client-side keys used with PayFlow.js and mobile SDKs for tokenization (ADR-005). Format: pk_live_xxx or pk_test_xxx.
- Used to initialize PayFlow.js: PayFlow.init('pk_live_xxx')
- Can only create tokens, cannot read or modify payment data
- Restricted by domain allowlist configured during onboarding (design-003)

### Restricted Keys
Keys with limited scopes for specific use cases (e.g., a key that can only read payment status but not create payments). Created via the merchant dashboard or API.

## Authentication Flow
1. Merchant includes API key in Authorization header
2. API gateway validates key format and extracts merchant_id
3. Key is verified against the key store (hashed with bcrypt, cached in Redis)
4. Request is annotated with merchant_id and granted scopes
5. Downstream services use the merchant_id for data isolation and the scopes for authorization

## Authorization Scopes
| Scope | Endpoints | Description |
|-------|-----------|-------------|
| payments:write | POST /v2/payments (api-001) | Create payments, capture, void |
| payments:read | GET /v2/payments (api-002) | Read payment status and details |
| refunds:write | POST /v2/payments/{id}/refunds (api-003) | Create refunds |
| refunds:read | GET /v2/payments/{id}/refunds | List refunds |
| webhooks:write | POST /v2/webhook_endpoints | Manage webhook subscriptions (design-002) |
| webhooks:read | GET /v2/webhook_endpoints | List webhook endpoints |
| customers:write | POST /v2/customers | Create and update customers |
| customers:read | GET /v2/customers | Read customer details |

## Key Rotation
Merchants can rotate API keys without downtime:
1. Generate a new key via dashboard or API
2. Both old and new keys are valid during the overlap period (configurable, default 24 hours)
3. Update application configuration to use the new key
4. Revoke the old key

Key rotation is recommended every 90 days. The merchant onboarding flow (design-003) configures initial keys and communicates rotation best practices.

## Inter-Service Authentication
Internal service-to-service communication uses mutual TLS with short-lived JWT tokens. The token vault (ADR-005) and payment pipeline (arch-002) services use additional RBAC policies:
- Only the payment pipeline can request detokenization
- Only the settlement engine (arch-003) can access settlement ledger APIs
- Fraud detection (arch-004) has read-only access to payment data

## Rate Limiting
Rate limits are applied per API key:
- Mutation endpoints (api-001, api-003): 100 req/s (default), up to 1,000 req/s (enterprise)
- Read endpoints (api-002): 200 req/s (default)
- Tokenization (publishable keys): 50 req/s per IP address
- Rate limit headers: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset

## Test Mode
Keys with the test prefix (sk_test_, pk_test_) operate against the sandbox environment. Test mode uses simulated provider responses and does not process real transactions. Test card numbers are documented in the integration guide (design-003).`,
		},

		// =====================================================================
		// pattern-001 through pattern-004: Code Patterns
		// =====================================================================
		{
			ID:    "pattern-001",
			Type:  "pattern",
			Title: "Pattern: Idempotent Request Processing",
			Scope: "project",
			Tags:  []string{"pattern", "idempotency", "reliability", "api", "concurrency"},
			Content: `# Pattern: Idempotent Request Processing

## Context
PayFlow's payment APIs must guarantee that retrying a request does not cause duplicate side effects, particularly duplicate charges. This pattern implements the idempotency strategy defined in ADR-003 and is used across all mutating endpoints (api-001, api-003).

## Pattern Overview
The idempotency layer intercepts all mutating requests before they reach the business logic. It uses a PostgreSQL-backed idempotency store (ADR-002) to track in-flight and completed requests.

## Implementation

### Data Model
Table: idempotency_keys
- merchant_id: UUID (from authentication, api-005)
- idempotency_key: VARCHAR(255) (from Idempotency-Key header)
- request_hash: BYTEA (SHA-256 of normalized request body)
- response_status: INTEGER (HTTP status code, NULL while in-flight)
- response_body: JSONB (response payload, NULL while in-flight)
- created_at: TIMESTAMP
- completed_at: TIMESTAMP (NULL while in-flight)
- PRIMARY KEY: (merchant_id, idempotency_key)

### Request Processing Flow

Step 1: Compute Request Hash
Normalize the request body (sort keys, remove whitespace) and compute SHA-256. This hash is used to detect request body mismatches on retry.

Step 2: Acquire Advisory Lock
Acquire a PostgreSQL advisory lock on hash(merchant_id || idempotency_key). This prevents concurrent processing of the same idempotency key without table-level locking.

Step 3: Check Idempotency Store
Query the idempotency_keys table:
- No record: This is a new request. Insert a record with NULL response fields and proceed to Step 4.
- Record exists, response NULL: Another request is in-flight. Return HTTP 409 Conflict with Retry-After: 1.
- Record exists, response present, request_hash matches: Return the stored response (idempotent replay).
- Record exists, response present, request_hash differs: Return HTTP 422 with error explaining that the idempotency key was already used with a different request body.

Step 4: Execute Business Logic
Process the request through the payment pipeline (arch-002), refund pipeline (design-004), or other business logic. The event sourcing system (ADR-001) records the resulting events.

Step 5: Store Response
Update the idempotency record with the response status and body. Release the advisory lock.

### Error Handling
If the business logic fails with a retryable error (provider timeout, database connection error), the idempotency record is deleted so the merchant can retry with the same key. Non-retryable errors (validation failures, declined transactions) are stored in the idempotency record.

### Cleanup
A background job purges idempotency records older than 24 hours. This balances storage costs against the merchant retry window. Records for failed-but-retryable requests are purged after 1 hour.

## Usage in PayFlow
- POST /v2/payments (api-001): Prevents duplicate payment creation
- POST /v2/payments/{id}/refunds (api-003): Prevents duplicate refund processing
- POST /v2/payments/{id}/capture: Prevents duplicate captures
- POST /v2/payments/{id}/void: Prevents duplicate voids

## Testing
Test with concurrent requests sharing the same idempotency key to verify advisory lock behavior. Test request body mismatch detection. Test in-flight request handling (409 response). Test cleanup job for expired records.

## Related
- ADR-003: Idempotency Keys for Payment APIs
- pattern-003: Event Sourcing Aggregate Pattern (idempotency at the event level)
- pattern-004: Saga Orchestration (idempotency across distributed operations)`,
		},
		{
			ID:    "pattern-002",
			Type:  "pattern",
			Title: "Pattern: Circuit Breaker Implementation",
			Scope: "project",
			Tags:  []string{"pattern", "circuit-breaker", "resilience", "fault-tolerance", "providers"},
			Content: `# Pattern: Circuit Breaker Implementation

## Context
PayFlow integrates with multiple payment providers that can experience outages or degradation. The November incident (meeting-004) demonstrated that without circuit breakers, a single provider's degradation causes cascading failures across the payment pipeline (arch-002). This pattern implements the circuit breaker strategy defined in ADR-004.

## Pattern Overview
Each payment provider integration has an independent circuit breaker that monitors request outcomes and transitions between three states: Closed (healthy), Open (failing, fast-fail), and Half-Open (probing for recovery).

## State Machine

### Closed (Normal Operation)
- All requests are forwarded to the provider
- Track success/failure counts in a sliding window (60 seconds)
- Transition to Open when: failure count >= 5 OR failure rate > 50% within the window

### Open (Fast Failure)
- All requests immediately return a ProviderUnavailable error
- The payment routing engine (arch-002) redirects eligible transactions to alternative providers
- After 30 seconds, transition to Half-Open

### Half-Open (Recovery Probe)
- Allow up to 3 probe requests through to the provider
- If 2 out of 3 probes succeed: transition to Closed
- If 2 out of 3 probes fail: transition back to Open (reset the 30-second timer)

## Implementation

### State Storage
Circuit breaker state is stored in Redis (ADR-002) to share state across all instances of the payment pipeline service. Redis keys:
- cb:{provider}:state - Current state (closed/open/half_open)
- cb:{provider}:failures - Sorted set of failure timestamps (sliding window)
- cb:{provider}:probes - Counter for half-open probe results
- cb:{provider}:opened_at - Timestamp when circuit opened

### Failure Classification
Not all errors indicate provider degradation. The circuit breaker distinguishes:

Counted as failures (indicate provider issues):
- HTTP 500, 502, 503, 504 responses
- Connection timeouts (>5 seconds per meeting-004 remediation)
- Connection refused or DNS resolution failures
- Request timeouts

Not counted as failures (indicate request issues):
- HTTP 400, 401, 403 (client errors - our request is malformed)
- HTTP 404 (resource not found - our data is wrong)
- HTTP 429 (rate limiting - handled separately with backoff)
- Payment declines (HTTP 402 or provider-specific decline codes)

This classification is critical. During Black Friday (meeting-003), Amir discovered that counting 4xx responses as failures caused premature circuit opening under high legitimate traffic.

### Event Publishing
Circuit state transitions are published as operational events to Kafka:
- circuit.opened: {provider, failure_count, failure_rate, window_start}
- circuit.half_open: {provider, opened_duration}
- circuit.closed: {provider, probe_results, recovery_duration}

These events trigger:
- PagerDuty alerts for circuit openings (SRE on-call notification)
- Fraud detection (arch-004) correlation to avoid false positives during failover
- Merchant dashboard notifications (provider health status)

### Integration with Payment Routing
When a circuit opens, the payment routing engine (arch-002) removes the provider from the active routing table. Transactions that can be processed by alternative providers are rerouted automatically. Transactions bound to the unavailable provider (captures on existing authorizations, refunds) are queued with a TTL.

The routing engine also considers circuit state when calculating routing scores, slightly penalizing providers that have recently recovered (circuit was open in the last 5 minutes) to allow gradual traffic ramp-up.

## Configuration
| Parameter | Default | BFCM Override |
|-----------|---------|---------------|
| Failure threshold | 5 consecutive | 50% rate in 60s |
| Open duration | 30 seconds | 30 seconds |
| Probe count | 3 | 5 |
| Probe success threshold | 2/3 | 3/5 |
| Sliding window | 60 seconds | 60 seconds |

BFCM overrides (meeting-003) use percentage-based failure detection to avoid false positives during high-volume periods.

## Related
- ADR-004: Circuit Breaker for Payment Provider Integration
- meeting-004: Incident Review - November Payment Failures
- arch-002: Payment Processing Pipeline
- meeting-003: Performance Review - Black Friday Prep`,
		},
		{
			ID:    "pattern-003",
			Type:  "pattern",
			Title: "Pattern: Event Sourcing Aggregate",
			Scope: "project",
			Tags:  []string{"pattern", "event-sourcing", "aggregate", "ddd", "payment-state"},
			Content: `# Pattern: Event Sourcing Aggregate

## Context
PayFlow uses event sourcing for payment state management (ADR-001). The aggregate pattern encapsulates the business rules for valid payment state transitions, ensuring that only legal operations can occur on a payment. This pattern is the foundation for the payment processing pipeline (arch-002) and the saga orchestration pattern (pattern-004).

## Pattern Overview
The Payment aggregate is the consistency boundary for payment operations. All state mutations go through the aggregate, which validates business rules, emits domain events, and applies those events to update its internal state.

## Aggregate Structure

### Payment Aggregate
Fields (derived from event replay):
- ID: Payment identifier
- MerchantID: Owning merchant
- Status: Current payment status (design-005)
- Amount: Original payment amount
- Currency: ISO 4217 currency code (arch-005)
- AmountCaptured: Total captured amount
- AmountRefunded: Total refunded amount
- ProviderID: Payment provider identifier
- ProviderTransactionID: Provider's transaction reference
- Events: Ordered list of domain events
- Version: Event sequence number (used for optimistic concurrency)

### Domain Events
- PaymentInitiated: Payment created with amount, currency, payment method
- PaymentAuthorized: Authorization successful with provider transaction ID
- PaymentAuthorizationFailed: Authorization declined with reason code
- PaymentCaptured: Funds captured (full or partial amount)
- PaymentCaptureFailed: Capture failed with reason
- PaymentVoided: Authorization cancelled
- PaymentRefundInitiated: Refund started for specified amount
- PaymentRefundCompleted: Refund confirmed by provider
- PaymentRefundFailed: Refund rejected by provider
- PaymentDisputed: Chargeback filed

## Command Handling

### CreatePayment Command
Preconditions: None (new aggregate)
Emits: PaymentInitiated
Side effects: Triggers payment pipeline (arch-002)

### AuthorizePayment Command
Preconditions: Status == pending
Emits: PaymentAuthorized or PaymentAuthorizationFailed
State change: Status -> authorized or failed

### CapturePayment Command
Preconditions: Status == authorized, capture_amount <= (Amount - AmountCaptured)
Emits: PaymentCaptured
State change: AmountCaptured += capture_amount, Status -> captured if fully captured

### VoidPayment Command
Preconditions: Status == authorized
Emits: PaymentVoided
State change: Status -> voided

### RefundPayment Command
Preconditions: Status in (captured, partially_refunded), refund_amount <= (AmountCaptured - AmountRefunded)
Emits: PaymentRefundInitiated
State change: Status -> partially_refunded (design-004)
Note: RefundCompleted is emitted after provider confirmation via saga (pattern-004)

## Event Storage
Events are persisted to the event store in PostgreSQL (ADR-002). Each event includes:
- aggregate_id: Payment ID
- sequence_number: Monotonically increasing per aggregate
- event_type: Domain event type name
- payload: JSON-serialized event data
- metadata: Correlation ID, causation ID, timestamp
- created_at: Server timestamp

Optimistic concurrency: When appending events, the expected sequence number is checked. If another process has appended events since the aggregate was loaded, a concurrency conflict is raised and the command is retried.

## Snapshotting
For payments with many events (e.g., subscriptions with monthly captures), we snapshot the aggregate state every 50 events. The snapshot stores the serialized aggregate state and the sequence number. Aggregate loading first checks for a snapshot, then replays only events after the snapshot.

## Projection
Events are published to Kafka for consumption by read model projectors (design-005), the webhook delivery system (design-002), the settlement engine (arch-003), and the fraud detection system (arch-004).

## Related
- ADR-001: Event Sourcing for Payment State Management
- pattern-004: Saga Orchestration for Distributed Transactions
- design-005: Payment Status Tracking
- arch-002: Payment Processing Pipeline`,
		},
		{
			ID:    "pattern-004",
			Type:  "pattern",
			Title: "Pattern: Saga Orchestration for Distributed Transactions",
			Scope: "project",
			Tags:  []string{"pattern", "saga", "orchestration", "distributed-transactions", "compensation"},
			Content: `# Pattern: Saga Orchestration for Distributed Transactions

## Context
PayFlow's payment processing spans multiple services and external providers. Traditional distributed transactions (2PC) are impractical due to provider API limitations and the need for high availability. The saga pattern coordinates multi-step operations with compensating actions for failure recovery. This pattern builds on the event sourcing aggregate (pattern-003) and is referenced by the payment pipeline (arch-002) and refund pipeline (design-004).

## Pattern Overview
PayFlow uses orchestration-based sagas where a central Saga Orchestrator coordinates the steps of a distributed transaction. The orchestrator maintains the saga state as an event-sourced aggregate (following pattern-003), ensuring the saga's own state is durable and recoverable.

## Saga Structure

### Payment Authorization Saga
The primary saga for processing a new payment (api-001):

Step 1: Validate Payment (Local)
- Validate request fields and merchant configuration
- Check idempotency (pattern-001)
- Compensation: None needed (no side effects)

Step 2: Evaluate Risk (Fraud Service)
- Call fraud detection system (arch-004) for risk evaluation
- If declined: Emit PaymentAuthorizationFailed, end saga
- If review: Hold payment in pending_review state, pause saga
- Compensation: Release any risk holds

Step 3: Reserve Funds (Provider)
- Submit authorization to selected provider (arch-002 routing)
- If declined: Emit PaymentAuthorizationFailed, end saga
- If provider timeout: Trigger circuit breaker evaluation (ADR-004, pattern-002)
- Compensation: Void the authorization if later steps fail

Step 4: Record Authorization (Local)
- Emit PaymentAuthorized event on payment aggregate (pattern-003)
- Update status projection (design-005)
- Enqueue webhook notification (design-002)
- Compensation: Emit compensating event (rare - only if post-auth processing fails)

### Refund Saga
Coordinates the refund flow (design-004, api-003):

Step 1: Validate Refund (Local)
- Verify payment state allows refund
- Check refund amount against remaining refundable balance
- Compensation: None

Step 2: Submit Refund to Provider
- Call the original provider's refund API
- The provider must match the original authorization provider
- Compensation: If provider confirms refund but our recording fails, reconciliation (arch-003) catches it

Step 3: Update Settlement (Settlement Service)
- Adjust merchant settlement balance (arch-003)
- Apply fee credits per merchant pricing agreement
- Compensation: Reverse settlement adjustment

Step 4: Record Refund (Local)
- Emit RefundCompleted event on payment aggregate (pattern-003)
- Enqueue webhook notification (design-002)
- Compensation: Emit compensating event

## Saga State Machine
Each saga instance transitions through states:
- started: Saga initiated, first step executing
- step_{n}_pending: Waiting for step N to complete
- step_{n}_completed: Step N succeeded, proceeding to N+1
- compensating: A step failed, executing compensating actions in reverse
- completed: All steps succeeded
- failed: Compensation completed (or compensation itself failed, requiring manual intervention)

## Implementation Details

### Orchestrator
The saga orchestrator runs as a dedicated service consuming commands from Kafka. Each saga type (payment_authorization, refund, capture) has a registered step definition with:
- Execute function: Performs the step's action
- Compensate function: Reverses the step's action
- Timeout: Maximum duration before treating as failed
- Retry policy: Number of retries with backoff before triggering compensation

### Durability
Saga state is persisted using event sourcing (pattern-003). Each step transition emits an event (SagaStepStarted, SagaStepCompleted, SagaStepFailed, SagaCompensationStarted). On service restart, incomplete sagas are recovered by replaying their event streams.

### Idempotency
Each saga step is idempotent (aligned with pattern-001). Steps use the saga ID + step number as a correlation key. Provider calls include the saga correlation ID in their idempotency key to prevent duplicate provider-side effects.

### Timeouts
Steps that do not complete within their timeout are treated as failed. The orchestrator publishes a timeout event and begins compensation. For provider calls, the timeout aligns with the circuit breaker configuration (ADR-004): 5 seconds for authorization, 10 seconds for refunds.

## Monitoring
- Saga completion rate and duration per saga type
- Compensation frequency (indicates reliability issues)
- Stuck sagas (in step_pending for longer than timeout * retry_count)
- Manual intervention queue for sagas that failed compensation

## Related
- pattern-003: Event Sourcing Aggregate Pattern
- pattern-001: Idempotent Request Processing
- arch-002: Payment Processing Pipeline
- design-004: Refund Pipeline
- ADR-004: Circuit Breaker for Payment Provider Integration`,
		},
	}
}
