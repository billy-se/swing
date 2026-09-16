import { getValidToken } from './auth';

export const fetchIt = async (endpoint: string, options: RequestInit = {}) => {
    const fullUrl = `${process.env.NEXT_PUBLIC_API_URL}${endpoint}`;
    console.log("Fetching URL:", fullUrl);

    const token = getValidToken();

    const headers: Record<string, string> = {
        ...(options.body ? { 'Content-Type': 'application/json' } : {} ),
        ...(options.headers as Record<string, string>),
    };

    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    try {
        const response = await fetch(fullUrl, {
            ...options,
            credentials: 'include', 
            headers,
        });

        return response;
    } catch (error) {
        console.error("FetchIt network error:", error);
        throw error;
    }
};