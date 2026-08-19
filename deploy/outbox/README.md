# W08 outbox deployment

`mysql.sql` is the explicit production schema for the default GORM store. Apply schema changes
through the deployment's normal migration process. `GORMStore.AutoMigrate` exists for controlled
dev/test environments and is not called automatically by the dispatcher.

Operational requirements:

- business changes and `EnqueueTx` must share the same active transaction;
- run more than one dispatcher only when the database supports row-level `FOR UPDATE` locking;
- configure `LeaseDuration >= ceil(BatchSize/Concurrency) * PublishTimeout`;
- alert on pending age, dead-letter count, and repeated lease/store errors;
- schedule explicit published-row retention via `RetentionStore.PurgePublished`;
- never auto-replay dead letters;
- consumer side effects must be idempotent by event ID because delivery is at-least-once.

The table stores the serialized event body because the dispatcher must eventually publish it.
Generic W07 diagnostics do not expose this body. Apply normal database encryption, backup, access
control, and retention policy to the outbox table according to the event data classification.
