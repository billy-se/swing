export interface Comment {
    id: string;
    user_id?: number | string;
    author: string;
    content: string;
    timestamp: string;
    score?: number;
    fire_count?: number;
    user_has_fired?: boolean;
    replies?: Comment[];
}

export interface Argument {
    id: number;
    author: string;
    title: string;
    content: string;
    logic_score: number;
    created_at: string;
    comments?: Comment[];
}

export interface NotificationItem {
    id?: string | number;
    argument_id?: number;
    comment_id?: number;
    type?: string;
    message?: string;
    content?: string;
    created_at?: string;
    is_read?: boolean;
    [key: string]: any;
}

export interface RawComment {
    id: string | number;
    user_id?: string | number;
    userId?: string | number;
    author_id?: string | number;
    author?: string;
    content: string;
    timestamp: string;
    score?: number;
    fire_count?: number;
    user_has_fired?: boolean;
    replies?: RawComment[];
    comments?: RawComment[];
}

export interface UserProfile  {
    id: number | null;
    username: string;
    role: string;
    logicScore: number;
}