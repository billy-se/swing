CREATE TABLE IF NOT EXISTS watchlist (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    argument_id INTEGER REFERENCES arguments(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, argument_id)
);

CREATE INDEX IF NOT EXISTS idx_watchlist ON watchlist(user_id, argument_id);