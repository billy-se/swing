export const fetchIt = async (endpoint: string, options: RequestInit = {}) => {
        const token = localStorage.getItem('user_token_swing');

        const headers: Record<string, string> = {
            ...options.headers as Record<string, string>
        };

        if (token && token !== 'null' && token !== 'undefined') {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const fullUrl = `${process.env.NEXT_PUBLIC_API_URL}${endpoint}`;
        console.log("Fetching URL:", fullUrl);

        const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}${endpoint}`, {
            ...options, headers,
        });

        return response;
    }