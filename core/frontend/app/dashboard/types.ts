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
    message?: string;
    content?: string;
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