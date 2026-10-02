'use client';

import { useState, useEffect, useRef, Suspense } from 'react';
import { Comment, Argument, NotificationItem, RawComment, UserProfile } from './types';
import { ReviewModal } from './reviewModal';
import { CreateArgumentModal } from './createModal';
import { api, setMemoryAccessToken, getMemoryAccessToken } from '@/app/dashboard/api';
import { useRouter, useSearchParams, usePathname } from 'next/navigation';
import { startTransition } from 'react';
import { ArgumentStatsResponse } from './types';

function Home() {
    const [isReviewOpen, setIsReviewOpen] = useState(false);
    const [selectedArgumentId, setSelectedArgumentId] = useState<null | number>(null);
    const [targetCommentId, setTargetCommentId] = useState<null | number | string>(null);
    const [argumentsList, setArgumentsList] = useState<Argument[]>([]);
    const [isCreateOpen, setIsCreateOpen] = useState(false);

    const [newTitle, setNewTitle] = useState("");
    const [newContent, setNewContent] = useState("");
    const [error, setError] = useState('');

    const [user, setUser] = useState<UserProfile | null>(null);

    const ws = useRef<WebSocket | null>(null);

    const [notifications, setNotifications] = useState<NotificationItem[]>([]);
    const [isNotificationOpen, setIsNotificationOpen] = useState(false);
    const notificationRef = useRef<HTMLDivElement>(null);

    const router = useRouter();
    const searchParams = useSearchParams();
    const pathname = usePathname();

    const activeTab = (searchParams.get('tab') as 'recent' | 'top' | 'watchlist') || 'recent';
    const [isLoading, setIsLoading] = useState(true);
    const [isAuthInitialized, setIsAuthInitialized] = useState(false); 

    const [cache, setCache] = useState<Record<string, Argument[]>>({});

    const page = Number(searchParams.get('page')) || 1;
    const highlightArgumentId = searchParams.get('argumentId');
    const targetArgumentId = searchParams.get('target_argument_id');
    const highlightCommentId = searchParams.get('commentId');
    const [hasMore, setHasMore] = useState(true);
    const [isLoadingList, setIsLoadingList] = useState(false);
    const [isLoadingMore, setIsLoadingMore] = useState(false);
    const LIMIT = 20;

    //const [stats, setStats] = useState<ArgumentStatsResponse | null>(null);

    const [pendingArguments, setPendingArguments] = useState<Argument[]>([]);
    const [newCount, setNewCount] = useState(0);
    const [maxPage, setMaxPage] = useState<number | null>(null);

    const mapComments = (commentsList: RawComment[]): Comment[] => {
        if (!Array.isArray(commentsList)) return [];

        const mapped = commentsList.map((comment) => ({
            id: String(comment.id),
            user_id: comment.user_id || comment.userId || comment.author_id || "",
            author: comment.author || "ANONYMOUS",
            content: comment.content,
            timestamp: comment.timestamp,
            score: comment.score ?? 0,
            fire_count: comment.fire_count ?? 0,
            user_has_fired: comment.user_has_fired ?? false,
            replies: mapComments(comment.replies || comment.comments || [])
        }));
        return mapped.reverse();
    };

    const loadArguments = async (reset = false, customPage?: number) => {
        const targetPage = customPage !== undefined ? customPage : (reset ? 1 : page);
        if (!reset && customPage === undefined && !hasMore) return;

        //if (!reset && !hasMore) return;
        if (reset) {
            setMaxPage(null);
        }

        if (reset && customPage === undefined && activeTab !== 'watchlist' && cache[activeTab]) {
            setArgumentsList(cache[activeTab]);
            //setPage(1);
            setHasMore(true);
            setMaxPage(null);
            return;
        }
        /*
        if (reset && cache[activeTab]) {
            setArgumentsList(cache[activeTab]);
            setPage(2);
            return;
        }*/

        try {
            if (reset || customPage !== undefined) {
                setIsLoadingList(true);
            } else {
                setIsLoadingMore(true);
            }

            const endpoint = activeTab === 'top'
                ? `/api/arguments/top?page=${targetPage}&limit=${LIMIT}`
                : activeTab === 'watchlist'
                    ? `/api/watchlist?page=${targetPage}&limit=${LIMIT}&t=${Date.now()}`
                    : `/api/arguments?page=${targetPage}&limit=${LIMIT}`;

            const res = await api.get(endpoint);
            const data = res.data;

            const items = Array.isArray(data) ? data : (data.arguments || data.watchlist || []);
            const totalItems = data.total ?? null;
            //if (!Array.isArray(data)) return;
            setHasMore(items.length === LIMIT);
            //if (items.length < LIMIT) setHasMore(false);
            if (totalItems !== null) {
                setMaxPage(Math.ceil(totalItems/LIMIT));
            } else if (items.length < LIMIT) {
                setMaxPage(targetPage);
            } else {
                setMaxPage(prev => (prev !== null && targetPage >= prev ? null : prev));
            }

            const initializedData = items.map((existingArguments: Argument) => ({
                ...existingArguments,
                comments: mapComments(existingArguments.comments || [])
            }));

            /*setArgumentsList(prevList => {
                if (reset || targetPage === 1) {
                    return initializedData;
                }

                const existingIds = new Set(prevList.map(item => item.id));
                const uniqueNewItems = initializedData.filter((item: Argument) => !existingIds.has(item.id));
                return [...prevList, ...uniqueNewItems];
            });*/
            setArgumentsList(prevList => {
                if (reset || customPage !== undefined || targetPage === 1) {
                    return initializedData;
                }

                const existingIds = new Set(prevList.map(item => item.id));
                const uniqueNewItems = initializedData.filter((item: Argument) => !existingIds.has(item.id));
                return [...prevList, ...uniqueNewItems];
            });

            /*if (customPage !== undefined) {
                setPage(customPage);
            } else if (reset) {
                setPage(1);
                setCache(prev => ({ ...prev, [activeTab]: initializedData }));
            } else {
                setPage(prev => prev+1);
            }*/

            if (reset) {
                setCache(prev => ({ ...prev, [activeTab]: initializedData }));
            }

            /*setCache(prev => ({ ...prev, [activeTab]: initializedData }));
            startTransition(() => {
                setArgumentsList(initializedData);
            });*/
        } catch (err) {
            console.error("Failed to fetch arguments:", err);
        } finally {
            //setIsLoading(false);
            setIsLoadingList(false);
            setIsLoadingMore(false);
        }
    };

    const handleToggleWatch = async (argumentId: number) => {
        try {
            const res = await api.post('/api/watchlist', { argument_id: argumentId });
            console.log("Watchlist API Response:", res.data);

            const isNowWatched = res.data.status === 'watched';

            setArgumentsList(prevList => {

                if (activeTab === 'watchlist' && !isNowWatched) {
                    return prevList.filter(arg => String(arg.id) !== String(argumentId));
                }

                return prevList.map(arg => {
                    if (String(arg.id) === String(argumentId)) {
                        return {
                            ...arg,
                            is_watched: isNowWatched
                        };
                    }
                    return arg;
                });
            });

            setCache({});
        }catch (err) {
            console.error("Failed to toggle watchlist: ", err);
        }
    }

    const handlePageChange = (newPage: number) => {
        const params = new URLSearchParams(searchParams.toString());
        params.set('page', String(newPage));
        router.push(`?${params.toString()}`);
    };

    const loadNotifications = async () => {
        try {
            const response = await api.get(`/api/notifications`);
            setNotifications(response.data.notifications || []);
        } catch (err) {
            console.error("Failed to fetch initial notifications", err);
        }
    };

    const markNotificationAsRead = async (notificationId: number) => {
        try {
            await api.patch(`/api/notifications/${notificationId}/read`);
            setNotifications(currentNotif =>
                currentNotif.map(notif => notif.id === notificationId ? { ...notif, is_read: true } : notif)
            );
        } catch (err) {
            console.error("Failed to mark notification as read", err);
        }
    };

    const handleLogout = async () => {
        try {
            await api.post('/api/logout');
        } catch (err) {
            console.error("Failed to logout on backend", err);
        } finally {
            sessionStorage.removeItem('viewer_username');
            sessionStorage.removeItem('user_role');
            setMemoryAccessToken('');
            
            setUser(null);
            
            setNotifications([]);
            setIsNotificationOpen(false);
            
            console.log("Session cleared. User is now a guest.");
            router.push('/');
        }
    };

    /*const handleLogoutAndReload = () => {

        sessionStorage.clear();
        localStorage.clear();
        
        window.location.href = '/login'; 
    };*/
    const hasInitialized = useRef(false);

    useEffect(() => {
        if (ws.current && (ws.current.readyState === WebSocket.OPEN || ws.current.readyState === WebSocket.CONNECTING)) return;
        if (hasInitialized.current) return;
        hasInitialized.current = true;

        let socket: WebSocket | null = null;
        let isMounted = true;
        let reconnectTimeout: NodeJS.Timeout;

        const initializeConnection = async () => {

            try {
            //await loadArguments();

            const sessionViewerName = sessionStorage.getItem('viewer_username');
            const sessionRole = sessionStorage.getItem('user_role');

            let fetchedRole = 'user';
            let token = getMemoryAccessToken();

            if (sessionRole === 'viewer' && sessionViewerName) {
                setUser({
                    id: null,
                    username: sessionViewerName,
                    role: 'viewer',
                    logicScore: 0
                });
                fetchedRole = 'viewer';
            } else {
            if (!token) {
                try {
                    const refreshRes = await api.post('/api/refresh');
                    token = refreshRes.data.access_token;
                    setMemoryAccessToken(token);

                    const profileRes = await api.get('/api/user/profile');
                    setUser(profileRes.data);
                } catch (err: any) {

                    if (err.response && err.response.status === 401) {
                        setUser(null);
                        setMemoryAccessToken(null);
                    } else {
                        console.warn("Network hiccup during session init, keeping previous state", err);
                    }
                    //console.log("No active session found. Running as guest");
                } finally {
                    setIsLoading(false);
                }
            }

            if (token) {
                try {
                    const res = await api.get('/api/user/profile');
                    const data = res.data;
                    
                    if (data.role) {
                        fetchedRole = data.role;
                    }
                    
                    setUser({
                        id: data.id || data.user_id || null,
                        username: data.username || 'ANONYMOUS',
                        role: fetchedRole,
                        logicScore: data.logic_score ?? 0
                    });

                    if (fetchedRole !== 'viewer'){
                        loadNotifications();
                    }
                } catch (err) {
                    console.error("Failed to load profile", err);
                    setUser(null);
                    //router.push('/');
                    return;
                }
            } else {
                setUser(null);
            }

            }

            if (fetchedRole !== 'viewer' && !token) {
                return;
            }

            try {
                const wsBaseUrl = process.env.NEXT_PUBLIC_WS_URL;
                const ticketRes = await api.post('/api/ws-ticket');
                const ticket = ticketRes.data.ticket;
                const wsUrl = `${wsBaseUrl}/ws?ticket=${ticket}`;

                if (!isMounted) return;

                socket = new WebSocket(wsUrl);
                ws.current = socket;

                socket.onerror = (error) => {
                    console.error("WebSocket Error Event Triggered:", error);
                };

                socket.onclose = (event) => {
                    console.warn(`WebSocket Closed! Code: ${event.code}, Reason: ${event.reason}`);
                    ws.current = null;

                    if (isMounted) {
                        console.log("Attempting to reconnect Websocket in 3s...");
                        //clearTimeout(reconnectTimeout);
                        reconnectTimeout = setTimeout(() => {
                            initializeConnection();
                        }, 3000);
                    }
                };

                socket.onmessage = (fromServerJson) => {
                    const messageJson = JSON.parse(fromServerJson.data);
                    console.log("WS Received:", messageJson);

                    if (messageJson.type === "NEW_ARGUMENT") {
                        const rawPayload = messageJson.payload || messageJson;
                        const incomingArgument: Argument = {
                            ...rawPayload,
                            comments: mapComments(rawPayload.comments || [])
                        };

                        /*setArgumentsList(existingArguments => {
                            if (existingArguments.some(item => item.id === incomingArgument.id)) return existingArguments;
                            return [incomingArgument, ...existingArguments];
                        });*/

                        setPendingArguments(existingArguments => {
                            if (existingArguments.some(item => item.id === incomingArgument.id)) return existingArguments;
                            return [incomingArgument, ...existingArguments];
                        });

                        setNewCount(notifBadge => notifBadge + 1);
                    } else if (messageJson.type === "NEW_COMMENT") {
                        const incomingComment: Comment = {
                            id: String(messageJson.payload.id),
                            user_id: messageJson.payload.user_id || messageJson.payload.userId || messageJson.payload.author_id,
                            author: messageJson.payload.author || "ANONYMOUS",
                            content: messageJson.payload.content,
                            timestamp: messageJson.payload.timestamp || (messageJson.payload.created_at ? messageJson.payload.created_at.split('.')[0].replace('T', ' ') : "Just now"),
                            score: messageJson.payload.score || 0,
                            fire_count: messageJson.payload.fire_count || 0,
                            replies: []
                        };

                        const argumentId = String(messageJson.payload.argument_id || messageJson.payload.arg_id);
                        const parentCommentId = messageJson.payload.parent_id;

                        const addReplyRecursive = (existingComments: Comment[]): Comment[] => {
                            return existingComments.map(commentItem => {
                                if (String(commentItem.id) === String(parentCommentId)) {
                                    if (commentItem.replies?.some(existingReply => existingReply.id === incomingComment.id)) return commentItem;
                                    return { ...commentItem, replies: [incomingComment, ...(commentItem.replies || [])] };
                                }
                                if (commentItem.replies && commentItem.replies.length > 0) {
                                    return { ...commentItem, replies: addReplyRecursive(commentItem.replies) };
                                }
                                return commentItem;
                            });
                        };

                        setArgumentsList(existingArguments => {
                            return existingArguments.map(argumentItem => {
                                if (String(argumentItem.id) === argumentId) {
                                    const existingComments = argumentItem.comments || [];
                                    if (parentCommentId == null || parentCommentId === undefined || parentCommentId === "root-id") {
                                        if (existingComments.some(commentItem => commentItem.id === incomingComment.id)) return argumentItem;
                                        return { ...argumentItem, comments: [incomingComment, ...existingComments] };
                                    }
                                    return { ...argumentItem, comments: addReplyRecursive(existingComments) };
                                }
                                return argumentItem;
                            });
                        });
                    } else if (messageJson.type === "SWING_SCORE_UPDATE") {
                        const messagePayload = messageJson.payload || messageJson;
                        const targetCommentId = String(messagePayload.comment_id || messagePayload.commentId || messagePayload.id);
                        const commentUserId = messagePayload.user_id;

                        const updateFireRecursive = (existingComments: Comment[]): Comment[] => {
                            return existingComments.map(commentItem => {
                                if (commentItem.id === targetCommentId) {
                                    const isCurrentUsersFire = commentUserId && user?.id && Number(commentUserId) === Number(user.id);
                                    return {
                                        ...commentItem,
                                        fire_count: messagePayload.fire_count !== undefined ? messagePayload.fire_count : commentItem.fire_count,
                                        user_has_fired: isCurrentUsersFire ? !commentItem.user_has_fired : commentItem.user_has_fired
                                    };
                                }
                                if (commentItem.replies && commentItem.replies.length > 0) {
                                    return { ...commentItem, replies: updateFireRecursive(commentItem.replies) };
                                }
                                return commentItem;
                            });
                        };

                        setArgumentsList(currentArgumentsList => {
                            return currentArgumentsList.map(argumentItem => {
                                const processedComments = updateFireRecursive(argumentItem.comments || []);
                                return { ...argumentItem, comments: processedComments };
                            });
                        });
                    } else if (messageJson.type === "NEW_NOTIFICATION") {
                        const incomingNotification = messageJson.payload || messageJson;
                        setNotifications(existingNotifications => [incomingNotification, ...existingNotifications]);
                    }
                };
            } catch (error) {
                console.error("Failed to fetch WebSocket ticket or connect:", error);
            }
        } catch (error) {
            console.error("Initialization error:", error);
        } finally {
            if (isMounted) {
                setIsLoading(false); 
                setIsAuthInitialized(true);
            }
        }
        };

        initializeConnection();

        const handleClickOutside = (clickEvent: MouseEvent) => {
            const notificationElement = notificationRef.current;
            const clickedTarget = clickEvent.target as Node;
            const isClickOutsideNotification = notificationElement && !notificationElement.contains(clickedTarget);

            if (isClickOutsideNotification) setIsNotificationOpen(false);
        };

        document.addEventListener('mousedown', handleClickOutside);

        return () => {
            isMounted = false;
            clearTimeout(reconnectTimeout);
            document.removeEventListener('mousedown', handleClickOutside);
            if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
                socket.close(1000, 'User session ended');
            }
            ws.current = null;
        };
    }, []);

    /*useEffect(() => {
        if (!isAuthInitialized) return;

        loadArguments(true, page);

        if (highlightArgumentId) {
            setTimeout(() => {
                const element = document.getElementById(`argument-${highlightArgumentId}`);
                if (element) {
                    element.scrollIntoView({ behavior: 'smooth', block: 'center' });
                    element.classList.add('ring-4', 'ring-emerald-500');
                }
            }, 500);
        }
        //setPage(1);
        //setHasMore(true);
        //loadArguments(true, 1);
    },[activeTab, page, isAuthInitialized]);*/

    const selectedArgument = (argumentsList || []).find((argumentItem) => argumentItem.id === selectedArgumentId) ?? null;
    
    const isViewer = user?.role === 'viewer';
    const isLoggedIn = user !== null;
    const currentUsername = user?.username ?? 'GUEST';
    const logicScore = user?.logicScore ?? 0;

    useEffect(() => { //under construction / suspect 1
        if (!isLoggedIn || isViewer) return;

        const interval = setInterval(async () => {
            try {
                await api.post('/api/heartbeat');
            } catch (err) {
                console.error("Heartbeat failed", err);
            }
        }, 15000);
        return () => clearInterval(interval);
    }, [isLoggedIn, isViewer]);

    useEffect(() => {
        const urlArgId = searchParams.get('argumentId');
        const urlCommentId = searchParams.get('commentId');

        if (urlArgId) {
            setSelectedArgumentId(Number(urlArgId));
            setTargetCommentId(urlCommentId ? Number(urlCommentId) : null);
            setIsReviewOpen(true);
        }
    }, [searchParams]);

    useEffect(() => {
        if (!isAuthInitialized) return;

        loadArguments(true, page);

        if (highlightArgumentId) {
            setTimeout(() => {
                const element = document.getElementById(`argument-${highlightArgumentId}`);
                if (element) {
                    element.scrollIntoView({ behavior: 'smooth', block: 'center' });
                    element.classList.add('ring-4', 'ring-emerald-500');
                }
            }, 500);
        }
    }, [activeTab, page, isAuthInitialized, highlightArgumentId]);

    useEffect(() => { //good
        const handleHashChange = () => {
            const hash = window.location.hash;
            const urlArgId = searchParams.get('argumentId');

            if (hash.startsWith('#argument-')) {
                const argId = hash.replace('#argument-', '');
                setSelectedArgumentId(Number(argId));
                setIsReviewOpen(true);
            } else if (!hash && isReviewOpen) {
                if (!urlArgId) {
                    setIsReviewOpen(false);
                }
            }
        };

        handleHashChange();

        window.addEventListener('hashchange', handleHashChange);
        return () => window.removeEventListener('hashchange', handleHashChange);
    }, [isReviewOpen]); // under construction

    const handleAddReply = (argumentId: string, incomingComment: { id: number; content: string; created_at: string; user_id?: number }) => {
        if (!selectedArgumentId) return;

        const processedComment: Comment = {
            id: String(incomingComment.id),
            user_id: incomingComment.user_id || user?.id || "",
            author: currentUsername,
            content: incomingComment.content,
            timestamp: incomingComment.created_at ? incomingComment.created_at.split('.')[0].replace('T', ' ') : "Just now",
            fire_count: 0,
            replies: []
        };

        setArgumentsList(existingArgument => existingArgument.map(processedArgument => {
            if (processedArgument.id === selectedArgumentId) {
                const currentComments = processedArgument.comments || [];

                if (argumentId === "root-id") {
                    if (currentComments.some(comment => comment.id === processedComment.id)) {
                        return processedArgument;
                    }
                    return {
                        ...processedArgument, comments: [processedComment, ...currentComments]
                    };
                }
                
                const addReplyRecursive = (commentList: Comment[]): Comment[] => {
                    return commentList.map(currentComment => {
                        if (currentComment.id === String(argumentId)) {
                            if (currentComment.replies?.some(replyItem => replyItem.id === processedComment.id)) return currentComment;
                            return { ...currentComment, replies: [processedComment, ...(currentComment.replies || [])] };
                        }
                        if (currentComment.replies && currentComment.replies.length > 0) {
                            return { ...currentComment, replies: addReplyRecursive(currentComment.replies) };
                        }
                        return currentComment;
                    });
                };
                
                return { ...processedArgument, comments: addReplyRecursive(currentComments) };
            }
            return processedArgument;
        }));
    };

    const handleCreateSubmit = async (submitEvent: React.SyntheticEvent) => {
        submitEvent.preventDefault();
        setError('');

        try {
            const response = await api.post(`/api/arguments`, { title: newTitle, content: newContent });
            const newArgument = response.data;

            setArgumentsList(existingArguments => {
                const processedArgument = {
                    ...newArgument,
                    comments: mapComments(newArgument.comments || [])
                };
                if (existingArguments.some(argumentItem => argumentItem.id === processedArgument.id)) return existingArguments;
                return [processedArgument, ...existingArguments];
            });

            setNewTitle("");
            setNewContent("");
            setIsCreateOpen(false);

        } catch (err: any) {
            const errorMsg = err.response?.data || err.message || 'Failed to create argument';
            setError(typeof errorMsg === 'string' ? errorMsg : JSON.stringify(errorMsg));
        }
    };

    const handleShowNewArguments = () => { //suspect 2
        setArgumentsList(existingArguments => [
            ...pendingArguments,
            ...existingArguments
        ]);
        setPendingArguments([]);
        setNewCount(0);

        window.scrollTo({ top: 0, behavior: 'smooth' });
    };

    return (
    <main className="h-[100dvh] w-full bg-black text-zinc-100 font-mono p-6 md:p-12 flex justify-center overflow-hidden box-border">
        {isLoading ? (
            <div className="w-full h-full flex flex-col justify-center items-center gap-3">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-emerald-500"></div>
                <span className="text-xs text-zinc-500 tracking-widest uppercase">Initializing Session...</span>
            </div>
        ) : (
        <div className="w-full max-w-6xl flex flex-col gap-4 h-[calc(100dvh-3rem)] md:h-[calc(100dvh-6rem)] overflow-hidden box-border">

            <header className="bg-zinc-950 border border-zinc-800 rounded-lg p-4 md:p-6 flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 shrink-0">
                <div className="flex items-center gap-3">
                    <div>
                        <h1 className="text-sm font-semibold tracking-widest uppercase text-zinc-200">
                            {currentUsername}
                        </h1>
                        <p className="text-[10px] text-zinc-500">Username</p>
                    </div>
                </div>

                <div className="flex items-center gap-6 text-xs border-t sm:border-t-0 border-zinc-800 pt-3 sm:pt-0 w-full sm:w-auto justify-between sm:justify-end">
                    {!isViewer && isLoggedIn && ( //suspect 3
                    <div
                        ref={notificationRef}
                        onClick={() => setIsNotificationOpen(!isNotificationOpen)}
                        className="cursor-pointer transition-opacity select-none relative hover:opacity-80"
                    >
                        <span className="text-zinc-500 block text-[10px]">NOTIFICATION</span>
                        {(() => {
                            const unreadCount = notifications.filter(n => !n.is_read).length;
                            return (
                                <span className={`font-bold ${unreadCount > 0 ? 'text-emerald-400' : 'text-red-500'}`}>
                                    {unreadCount} 🐌
                                </span>
                            );
                        })()}

                        {isNotificationOpen && (
                            <div className="absolute right-0 mt-2 w-64 bg-zinc-900 border border-zinc-800 rounded shadow-lg p-2 z-50 max-h-64 overflow-y-auto">
                                {notifications.length > 0 ? (
                                    notifications.map((notif) => (
                                        <div
                                            key={notif.id}
                                            onClick={async () => {
                                                if (!notif.is_read && notif.id !== undefined && notif.id !== null) {
                                                    await markNotificationAsRead(Number(notif.id));
                                                    setNotifications(prev =>
                                                        prev.map(n => n.id === notif.id ? { ...n, is_read: true } : n)
                                                    );
                                                }

                                                if (notif.argument_id !== undefined && notif.argument_id !== null) {
                                                    setIsNotificationOpen(false);

                                                    setSelectedArgumentId(Number(notif.argument_id));
                                                    setTargetCommentId(notif.comment_id || null);
                                                    setIsReviewOpen(true);
                                                    
                                                    const currentParams = new URLSearchParams(window.location.search);
                                                    currentParams.set('argumentId', String(notif.argument_id));
                                                    if (notif.comment_id) {
                                                        currentParams.set('commentId', String(notif.comment_id));
                                                    } else {
                                                        currentParams.delete('commentId');
                                                    }

                                                    window.history.replaceState(null, '', `?${currentParams.toString()}`);
                                                }
                                            }}
                                            className={`text-[11px] py-2 px-2 border-b border-zinc-800 last:border-0 cursor-pointer transition-colors ${notif.is_read ? 'text-zinc-400 bg-transparent' : 'text-zinc-100 bg-zinc-800 font-semibold'}`}
                                        >
                                            <p>{notif.content}</p>
                                            <span className="text-[9px] text-zinc-500 block mt-0.5">{notif.created_at || "Just now"}</span>
                                        </div>
                                    ))
                                ) : (
                                    <div className="text-zinc-500 text-[11px] py-1 text-center">No new notifications</div>
                                )}
                            </div>
                        )}
                    </div>
                    )}
                    {!isViewer && isLoggedIn && (
                    <div>
                        <span className="text-zinc-500 block text-[10px]">LOGIC SCORE</span>
                        <span className="text-emerald-400 font-bold">{logicScore}</span>
                    </div>
                    )}
                </div>
                
            </header>
            
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1 min-h-0 overflow-hidden">
                <div className="lg:col-span-2 flex flex-col gap-4 h-full min-h-0">
                    <div className="flex justify-between items-center bg-zinc-950 border border-zinc-800 rounded-lg p-3 text-xs text-zinc-400 shrink-0">
                        <span className="text-zinc-200 font-bold tracking-wider px-1">ACTIVE DEBATES</span>
                        <div className="flex gap-10 bg-zinc-900/60 p-1 rounded-lg border border-zinc-800">
                            <button 
                                onClick={() => router.push('?tab=recent&page=1')}
                                className={activeTab === 'recent' ? "text-emerald-400 underline font-semibold" : "text-zinc-400 hover:text-zinc-200"}
                                >
                                    Recent</button>

                            <button 
                                onClick={() => router.push('?tab=top&page=1')}
                                className={activeTab === 'top' ? "text-emerald-400 underline font-semibold" : "text-zinc-400 hover:text-zinc-200"}>Top</button>
                            {!isViewer && <button 
                                onClick={() => router.push('?tab=watchlist&page=1')}
                                className={activeTab === 'watchlist' ? "text-emerald-400 underline font-semibold" : "text-zinc-400 hover:text-zinc-200"}
                                >
                                    WatchList</button>}
                            
                        </div>
                    </div>
                        <div className="flex-1 min-h-0 overflow-y-auto space-y-4 pr-2 pb-6 relative [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
                        {newCount > 0 && (
                            <div className="absolute top-2 left-0 right-0 flex justify-center z-30 pointer-events-none">
                                <button
                                    onClick={handleShowNewArguments}
                                    className="pointer-events-auto bg-black/90 backdrop-blur-md border border-blue-500/40 text-blue-400 px-3.5 py-1 rounded-full shadow-lg text-[11px] font-mono hover:bg-blue-950/80 hover:border-blue-400 transition-all flex items-center gap-1.5"
                                >
                                    <span>↑</span> {newCount} new argument{newCount > 1 ? 's' : ''}
                                </button>
                            </div>
                        )}

                        {(argumentsList || []).length > 0 ? (
                            <>
                                {(argumentsList || []).map((check) => (
                                    <div key={check.id} className="bg-zinc-950 border border-zinc-800 rounded-lg p-5 flex flex-col gap-4 hover:border-zinc-700 transition-colors">
                                        <div className="flex justify-between items-center text-[10px] text-zinc-500">
                                            <span>AUTHOR: [{check.author}]</span>
                                            {!isViewer && <button 
                                                onClick={() => handleToggleWatch(check.id)}
                                                className={`px-2.5 py-1 rounded border text-xs font-medium transition-colors flex items-center gap-1.5 ${
                                                    check.is_watched 
                                                        ? 'bg-emerald-950/50 border-emerald-500/50 text-emerald-400' 
                                                        : 'bg-zinc-900 border-zinc-800 text-zinc-400 hover:text-zinc-200 hover:border-zinc-700'
                                                }`}
                                            >
                                                <span>{check.is_watched ? '★' : '☆'}</span>
                                            </button>}
                                        </div>
                                        <h2 className="text-sm font-semibold text-zinc-100">
                                            {check.title}
                                        </h2>
                                        <p className="text-xs text-zinc-400 leading-relaxed whitespace-pre-wrap">
                                            {check.content}
                                        </p>
                                        <div className="flex justify-between items-center pt-3 border-t border-zinc-900 text-xs">
                                            <span className="text-emerald-400">Logic Score: +{check.logic_score}</span>
                                            <button className="bg-zinc-900 hover:bg-zinc-800 text-zinc-200 px-3 py-1 rounded border border-zinc-800 text-xs transition-colors"
                                                onClick={() => {
                                                    //router.push(`?tab=${activeTab}&page=${page}&argumentId=${check.id}`);
                                                    window.history.pushState(null, '', `#argument-${check.id}`);

                                                    setSelectedArgumentId(check.id);
                                                    setTargetCommentId(null);
                                                    setIsReviewOpen(true);
                                                }}>
                                                Review Argument
                                            </button>
                                        </div>
                                    </div>
                                ))}
                                {/*
                                {hasMore && (
                                    <div className="flex justify-center pt-2">
                                        <button
                                            onClick={() => loadArguments(false)}
                                            disabled={isLoadingMore}
                                            className="px-4 py-2 bg-zinc-900 border border-zinc-800 text-zinc-300 hover:text-emerald-400 hover:border-emerald-500/50 rounded transition-colors text-xs font-mono disabled:opacity-50"
                                        >
                                            {isLoadingMore ? "Loading..." : "Load More"}
                                        </button>
                                    </div>
                                )}*/}
                                <div className="flex justify-between items-center pt-4 border-t border-zinc-900 text-xs font-mono shrink-0">
                                    <button
                                        onClick={() => {
                                            const chunkSize = 10;
                                            const targetPage = Math.max(1, page - chunkSize);
                                            //loadArguments(false, targetPage);
                                            router.push(`?tab=${activeTab}&page=${targetPage}`);
                                        }}
                                        disabled={page <= 1 || isLoadingList}
                                        className="px-3 py-1.5 bg-zinc-900 border border-zinc-800 text-zinc-300 hover:text-emerald-400 hover:border-emerald-500/50 rounded transition-colors disabled:opacity-30 disabled:hover:text-zinc-300 disabled:hover:border-zinc-800"
                                    >
                                        &larr; Prev
                                    </button>

                                    <div className="flex items-center gap-1.5 flex-wrap justify-center">
                                        {(() => {
                                            const chunkSize = 10;
                                            const currentChunk = Math.floor((page - 1) / chunkSize);
                                            const startPage = currentChunk * chunkSize + 1;
                                            
                                            const pages: number[] = [];
                                            
                                            for (let i = 0; i < chunkSize; i++) {
                                                const p = startPage + i;
                                                
                                                if (maxPage !== null && p > maxPage) {
                                                    break;
                                                }
                                                
                                                pages.push(p);
                                            }

                                            return pages.map((p) => {
                                                const isActive = page === p;
                                                return (
                                                    <button
                                                        key={p}
                                                        onClick={() => router.push(`?tab=${activeTab}&page=${p}`)}
                                                        disabled={isLoadingList}
                                                        className={`w-7 h-7 flex items-center justify-center rounded border text-xs transition-colors ${
                                                            isActive 
                                                                ? 'bg-emerald-950/50 border-emerald-500/50 text-emerald-400 font-bold' 
                                                                : 'bg-zinc-900 border-zinc-800 text-zinc-400 hover:text-zinc-200 hover:border-zinc-700'
                                                        }`}
                                                    >
                                                        {p}
                                                    </button>
                                                );
                                            });
                                        })()}
                                    </div>

                                    <button
                                        onClick={() => {
                                            const chunkSize = 10;
                                            const nextTarget = page + chunkSize;
                                            const targetPage = maxPage !== null ? Math.min(maxPage, nextTarget) : nextTarget;
                                            //loadArguments(false, targetPage);
                                            router.push(`?tab=${activeTab}&page=${targetPage}`);
                                        }}
                                        disabled={!hasMore || (maxPage !== null && page >= maxPage) || isLoadingList}
                                        className="px-3 py-1.5 bg-zinc-900 border border-zinc-800 text-zinc-300 hover:text-emerald-400 hover:border-emerald-500/50 rounded transition-colors disabled:opacity-30 disabled:hover:text-zinc-300 disabled:hover:border-zinc-800"
                                    >
                                        Next &rarr;
                                    </button>
                                </div>
                            </>
                        ) : activeTab === 'watchlist' ? (
                            <div className="text-xs text-zinc-500 text-center">No Watchlist added yet</div>
                        ) : activeTab === 'top' ? (
                            <div className="text-xs text-zinc-500 text-center">No Argument yet</div>
                        ) : (
                            <div className="text-xs text-zinc-500 text-center">No Argument yet</div>
                        )}
                    </div>
                </div>

                <div className="flex flex-col gap-6 h-full min-h-0 overflow-y-auto [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
                    {isLoggedIn && !isViewer && (
                        <div className="bg-zinc-950 border border-zinc-800 rounded-lg p-5 flex flex-col gap-4 shrink-0">
                            <h2 className="text-xs font-bold tracking-wider uppercase text-zinc-200">
                                SUBMIT FOR REVIEW
                            </h2>
                            <p className="text-xs text-zinc-500">
                                Drop the fluff and state your case.
                            </p>
                            <button className="w-full bg-emerald-600 hover:bg-emerald-500 text-black font-semibold py-2 rounded text-xs transition-colors"
                                onClick={() => setIsCreateOpen(true)}>
                                Create an Argument
                            </button>
                        </div>
                    )}

                    <div className="bg-zinc-950 border border-zinc-800 rounded-lg p-5 flex flex-col gap-3 shrink-0">
                        <h2 className="text-xs font-bold tracking-wider uppercase text-zinc-200">
                            Stats:
                        </h2>
                        <div className="flex flex-col gap-2 text-xs text-zinc-400">
                            <div className="flex justify-between">
                                <span>Peer Reviews Given:</span>
                                <span className="text-zinc-200">14</span>
                            </div>
                            <div className="flex justify-between">
                                <span>Logic Consensus Rate:</span>
                                <span className="text-emerald-400">89%</span>
                            </div>
                            <div className="flex justify-between">
                                <span>Anonymity Integrity:</span>
                                <span className="text-blue-400">Secure</span>
                            </div>
                        </div>
                    </div>

                    {!isViewer && isLoggedIn && (
                        <div className="flex items-center gap-4 shrink-0">
                            <button 
                                onClick={handleLogout}
                                className="bg-zinc-900 hover:bg-zinc-800 text-red-400 hover:text-red-300 px-3 py-1 rounded border border-zinc-800 transition-colors"
                            >
                                Logout
                            </button>
                        </div>
                    )}
                </div>
            </div>
            
            <ReviewModal
                isReviewOpen={isReviewOpen}
                selectedArgument={selectedArgument}
                selectedArgumentId={selectedArgumentId}
                targetCommentId={targetCommentId}
                setIsReviewOpen={setIsReviewOpen}
                handleAddReply={handleAddReply}
            />

            {isLoggedIn && !isViewer && (
                <CreateArgumentModal
                    isCreateOpen={isCreateOpen}
                    error={error}
                    newTitle={newTitle}
                    newContent={newContent}
                    setNewTitle={setNewTitle}
                    setNewContent={setNewContent}
                    setIsCreateOpen={setIsCreateOpen}
                    onSubmit={handleCreateSubmit}
                />
            )}
        </div>
        )}
    </main>
);
}

export default function Dashboard() {
    return (
        <Suspense fallback={<div className="p-8 text-zinc-500 text-center">Loading...</div>}>
            <Home />
        </Suspense>
    );
}