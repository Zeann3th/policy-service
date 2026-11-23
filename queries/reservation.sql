-- name: CreateReservation :one
INSERT INTO reservations
  (id, request_id, status, transaction_type, channel, currency, amount,
   sender_wallet_id, sender_wallet_type, sender_tier,
   receiver_wallet_id, receiver_wallet_type, receiver_tier)
VALUES
  (sqlc.arg(id), sqlc.arg(request_id), sqlc.arg(status), sqlc.arg(transaction_type), sqlc.arg(channel),
   sqlc.arg(currency), sqlc.arg(amount),
   sqlc.arg(sender_wallet_id), sqlc.arg(sender_wallet_type), sqlc.arg(sender_tier),
   sqlc.arg(receiver_wallet_id), sqlc.arg(receiver_wallet_type), sqlc.arg(receiver_tier))
RETURNING created_at;

-- name: UpdateReservationStatus :execrows
UPDATE reservations
SET status = sqlc.arg(status), result = sqlc.narg(result)
WHERE id = sqlc.arg(id) AND created_at = sqlc.arg(created_at) AND status = sqlc.arg(from_status);

-- name: LockWallet :exec
SELECT pg_advisory_xact_lock(hashtextextended(CAST(sqlc.arg(wallet_id) AS text), 0));

-- name: GetUsageValue :one
SELECT value FROM usages
WHERE wallet_id = sqlc.arg(wallet_id) AND policy_type_id = sqlc.arg(policy_type_id)
  AND transaction_type = sqlc.arg(transaction_type) AND channel = sqlc.arg(channel)
  AND direction = sqlc.arg(direction) AND window_type = sqlc.arg(window_type)
  AND window_start = sqlc.arg(window_start);

-- name: AddUsage :one
INSERT INTO usages
  (wallet_id, policy_type_id, transaction_type, channel, direction, window_type, window_start, value)
VALUES
  (sqlc.arg(wallet_id), sqlc.arg(policy_type_id), sqlc.arg(transaction_type), sqlc.arg(channel),
   sqlc.arg(direction), sqlc.arg(window_type), sqlc.arg(window_start), sqlc.arg(delta))
ON CONFLICT (wallet_id, policy_type_id, transaction_type, channel, direction, window_type, window_start)
DO UPDATE SET value = usages.value + EXCLUDED.value
RETURNING value;

-- name: CreateUsageLedger :exec
INSERT INTO usage_ledger
  (reservation_id, reservation_created_at, entry_type, wallet_id, policy_type_id, transaction_type,
   channel, direction, window_type, window_start, delta,
   policy_id, policy_type_code, operator, limit_value, value_after)
VALUES
  (sqlc.arg(reservation_id), sqlc.arg(reservation_created_at), sqlc.arg(entry_type), sqlc.arg(wallet_id),
   sqlc.arg(policy_type_id), sqlc.arg(transaction_type), sqlc.arg(channel), sqlc.arg(direction),
   sqlc.arg(window_type), sqlc.arg(window_start), sqlc.arg(delta),
   sqlc.narg(policy_id), sqlc.arg(policy_type_code), sqlc.narg(operator), sqlc.narg(limit_value),
   sqlc.arg(value_after));
