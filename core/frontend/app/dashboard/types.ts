export interface Comment {
    id: string;
    user_id?: number;
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