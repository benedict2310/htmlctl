CREATE UNIQUE INDEX IF NOT EXISTS verification_tokens_token_hash_unique_idx
  ON verification_tokens (token_hash);
