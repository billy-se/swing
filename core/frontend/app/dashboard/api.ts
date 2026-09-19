import axios from 'axios';

let memoryAccessToken: string | null = null;

export const setMemoryAccessToken = (token: string | null) => {
    memoryAccessToken = token;
};

export const getMemoryAccessToken = () => {
    return memoryAccessToken;
}

export const api = axios.create({
    baseURL: process.env.NEXT_PUBLIC_API_URL,
    withCredentials: true,
});

api.interceptors.request.use(
    (config) => {
        if (memoryAccessToken) {
            config.headers.Authorization = `Bearer ${memoryAccessToken}`;
        }
        return config;
    },
    (error) => Promise.reject(error)
);

let isRefreshing = false;
let failedQueue: any[] = [];

const processQueue = (error: any, token: string | null = null) => {
    failedQueue.forEach((prom) => {
        if (error) {
            prom.reject(error);
        } else {
            prom.resolve(token);
        }
    });
    failedQueue = [];
};

api.interceptors.response.use(
    (response) => response,
    async (error) => {
        const originalRequest = error.config;

        if (originalRequest.url?.includes('/api/login') || originalRequest.url?.includes('/api/refresh') || originalRequest.url?.includes('/api/viewer'))
        {return Promise.reject(error)};

        if (error.response?.status === 401 && !originalRequest._retry) {
            
            if (originalRequest.url?.includes('/api/refresh')) {
                memoryAccessToken = null;
                return Promise.reject(error);
            }
            if (isRefreshing) {
                return new Promise((resolve, reject) => {
                    failedQueue.push({ resolve, reject });
                })
                .then((token) => {
                    originalRequest.headers['Authorization'] = `Bearer ${token}`;
                    return api(originalRequest);
                })
                .catch((err) => {
                    return Promise.reject(err);
                });
            }

            originalRequest._retry = true;
            isRefreshing = true;

            try {
                const res = await axios.post(
                    `${process.env.NEXT_PUBLIC_API_URL}/api/refresh`,
                    {},
                    { withCredentials: true }
                );

                const newAccessToken = res.data.access_token;
                setMemoryAccessToken(newAccessToken);

                originalRequest.headers['Authorization'] = `Bearer ${newAccessToken}`;
                processQueue(null, newAccessToken);
                isRefreshing = false;

                return api(originalRequest);
            } catch (refreshError: any) {
                console.log("FULL ERROR OBJECT:", refreshError.response);
                processQueue(refreshError, null);
                isRefreshing = false;
                memoryAccessToken = null;
                return Promise.reject(refreshError);
            }
        }

        return Promise.reject(error);
    }
);