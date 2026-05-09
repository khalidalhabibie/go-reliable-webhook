ALTER TABLE webhook_deliveries
    ADD COLUMN lock_token UUID NULL;
