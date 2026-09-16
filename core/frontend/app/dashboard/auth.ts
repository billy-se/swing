import { getMemoryAccessToken } from "./api";

export const getValidToken = () => {
    const rawToken = getMemoryAccessToken();
    if(!rawToken || rawToken === 'null' || rawToken === 'undefined' ) return null;

    try {
        const payloadBase64 = rawToken.split('.')[1];
        const decodedPayload = JSON.parse(atob(payloadBase64));

        if (decodedPayload.exp && decodedPayload.exp * 1000 < Date.now()) {
            console.warn("Token has expired. Clearing from storage.");
            localStorage.removeItem('user_token_swing');
            return null;
        }

        return rawToken;
    } catch (err) {
        localStorage.removeItem('user_token_swing');
        return null;
    }
};