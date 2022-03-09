client {
  enabled = true

  carbon {
    provider = "aws"
    region   = "us-east-1"

    aws {
      access_key_id     = "access-key-id"
      secret_access_key = "secret-access-key"
      session_token     = "session-token"
    }
  }
}
