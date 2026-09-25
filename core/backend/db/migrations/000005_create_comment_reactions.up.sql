CREATE TABLE IF NOT EXISTS comment_reactions(
    id SERIAL PRIMARY KEY,
    comment_id INT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction_type VARCHAR(50) NOT NULL DEFAULT 'fire',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_comment_user_reaction UNIQUE (comment_id, user_id)
);  

CREATE INDEX IF NOT EXISTS idx_comment_reactions_optimized ON comment_reactions(comment_id, reaction_type, user_id);