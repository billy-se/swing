CREATE TABLE IF NOT EXISTS arguments (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    author VARCHAR(50) NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    logic_score INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_get_argument ON arguments(created_at);
CREATE INDEX IF NOT EXISTS idx_argument_logic_score ON arguments(logic_score DESC);