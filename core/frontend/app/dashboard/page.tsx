'use client';

import { useState, useEffect, useRef } from 'react';
import { Comment, Argument, NotificationItem, RawComment } from './types';
import { ReviewModal } from './reviewModal';
import { CreateArgumentModal } from './createModal';
import { fetchIt } from './fetchIt';
import { getValidToken } from './auth';
import { api, setMemoryAccessToken } from '@/app/dashboard/api';

export default function Home() {
    const [isReviewOpen, setIsReviewOpen] = useState(false);
    const [selectedArgumentId, setSelectedArgumentId] = useState<null | number>(null);
    const [targetCommentId, setTargetCommentId] = useState<null | number | string>(null);
    const [argumentsList, setArgumentsList] = useState<Argument[]>([]);
    const [isCreateOpen, setIsCreateOpen] = useState(false);

    const [newTitle, setNewTitle] = useState("");
    const [newContent, setNewContent] = useState("");

    const [error, setError] = useState('');
    const [isLoggedIn, setIsLoggedIn] = useState(false);
    const [currentUsername, setCurrentUsername] = useState('GUEST');
    const [logicScore, setLogicScore] = useState(0);

    const ws = useRef<WebSocket | null>(null);

    const [notifications, setNotifications] = useState<NotificationItem[]>([]);
    const [isNotificationOpen, setIsNotificationOpen] = useState(false);
    const notificationRef = useRef<HTMLDivElement>(null);

    const unreadCount = notifications.filter(n => !n.is_read).length;

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

    const loadArguments = async () => {
        try {
            const res = await fetchIt(`/api/arguments`);
            if (!res.ok) throw new Error('Failed to fetch arguments');

            const data = await res.json();
            if (!Array.isArray(data)) return;

            const initializedData = data.map((existingArguments: Argument) => ({
                ...existingArguments,
                comments: mapComments(existingArguments.comments || [])
            }));

            setArgumentsList(initializedData);
        } catch (err) {
            console.error("Failed to fetch arguments:", err);
        }
    };

    const loadNotifications = async () => {
        try {
            const res = await fetchIt(`/api/notifications`);
            if (res.ok) {
                const notifData = await res.json();
                setNotifications(notifData.notifications || []);
            }
        } catch (err) {
            console.error("Failed to fetch initial notifications", err);
        }
    };

    const markNotificationAsRead = async (notificationId: number) => {
        try {
            const res = await fetchIt(`/api/notifications/${notificationId}/read`, {
                method: 'PATCH',
            });
            if (!res.ok) throw new Error('Failed to mark notification as read');

            setNotifications(currentNotif =>
                currentNotif.map(notif => notif.id === notificationId ? { ...notif, is_read: true } : notif)
            );
        } catch (err) {
            console.error("Failed to mark notification as read", err);
        }
    };

    useEffect(() => {
        if (ws.current && (ws.current.readyState === WebSocket.OPEN || ws.current.readyState === WebSocket.CONNECTING)) return;

        let socket: WebSocket | null = null;
        let isMounted = true;

        const initializeConnection = async () => {
            let token = getValidToken();

            if (!token) {
                try {
                    const refreshRes = await api.post('/api/refresh');
                    token = refreshRes.data.access_token;
                    setMemoryAccessToken(token);
                } catch (err) {
                    console.log("No active session found. Running as guest");
                }
            }

            loadArguments();
            loadNotifications();

            if (token) {
                setIsLoggedIn(true);
                try {
                    const res = await api.get('/api/user/profile');
                    const data = res.data;
                    if (data.username) setCurrentUsername(data.username);
                    if (data.logic_score !== undefined) setLogicScore(data.logic_score);
                } catch (err) {
                    console.error("Failed to load profile", err);
                }
            } else {
                setIsLoggedIn(false);
                setCurrentUsername('GUEST');
            }

            try {
                const wsBaseUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:2026';
                let wsUrl = `${wsBaseUrl}/ws`;

                if (token) {
                    const ticketRes = await api.post('/api/ws-ticket');
                    const ticket = ticketRes.data.ticket;
                    wsUrl = `${wsBaseUrl}/ws?ticket=${ticket}`;
                }

                if (!isMounted) return;

                console.log("Attempting to connect to:", wsUrl);
                socket = new WebSocket(wsUrl);
                ws.current = socket;

                socket.onopen = () => {
                    console.log("WebSocket successfully connected via ticket!");
                };

                socket.onerror = (error) => {
                    console.error("WebSocket error occurred:", error);
                };

                socket.onclose = (event) => {
                    console.log("WebSocket closed:", event.reason);
                };

                socket.onmessage = (fromServerJson) => {
                    const messageJson = JSON.parse(fromServerJson.data);

                    if (messageJson.type === "NEW_ARGUMENT") {
                        const rawPayload = messageJson.payload || messageJson;
                        const incomingArgument: Argument = {
                            ...rawPayload,
                            comments: mapComments(rawPayload.comments || [])
                        };

                        setArgumentsList(existingArguments => {
                            if (existingArguments.some(item => item.id === incomingArgument.id)) return existingArguments;
                            return [incomingArgument, ...existingArguments];
                        });

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

                        const currentUserId = (() => {
                            try {
                                const currentStoredToken = localStorage.getItem('user_token_swing');
                                if (!currentStoredToken) return null;

                                const tokenPayload = JSON.parse(atob(currentStoredToken.split('.')[1]));
                                return tokenPayload.user_id || tokenPayload.id;
                            } catch {
                                return null;
                            }
                        })();

                        const updateFireRecursive = (existingComments: Comment[]): Comment[] => {
                            return existingComments.map(commentItem => {
                                if (commentItem.id === targetCommentId) {
                                    const isCurrentUsersFire = commentUserId && Number(commentUserId) === Number(currentUserId);

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
            document.removeEventListener('mousedown', handleClickOutside);
            if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
                socket.close(1000, 'User session ended');
            }
            ws.current = null;
        };
    }, []);

    const selectedArgument = (argumentsList || []).find((argumentItem) => argumentItem.id === selectedArgumentId) ?? null;

    const handleAddReply = (argumentId: string, incomingComment: { id: number; content: string; created_at: string; user_id?: number }) => {
        if (!selectedArgumentId) return;

        const currentUserId = (() => {
            try {
                const token = localStorage.getItem('user_token_swing');

                if (!token) return null;
                const payload = JSON.parse(atob(token.split('.')[1]));

                return payload.user_id || payload.id;
            } catch {
                return null;
            }
        })();

        const processedComment: Comment = {
            id: String(incomingComment.id),
            user_id: incomingComment.user_id || currentUserId,
            author: currentUsername,
            content: incomingComment.content,
            timestamp: incomingComment.created_at ? incomingComment.created_at.split('.')[0].replace('T', ' ') : "Just now",
            fire_count: 0,
            replies: []
        };

        setArgumentsList(existingArgument => existingArgument.map(processedArgument => {
            if (processedArgument.id === selectedArgumentId) {
                const currentComments = processedArgument.comments || [];

                if (argumentId == "root-id") {

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
            const response = await fetchIt(`/api/arguments`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ title: newTitle, content: newContent }),
            });

            if (!response.ok) {
                const errorText = await response.text();
                setError(errorText);
                return;
            }

            const newArgument = await response.json();

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
            setError(err.message);
        }
    };

    return (
        <main className="min-h-screen bg-black text-zinc-100 font-mono p-6 md:p-12 flex justify-center">
            <div className="w-full max-w-6xl flex flex-col gap-6">

                <header className="bg-zinc-950 border border-zinc-800 rounded-lg p-4 md:p-6 flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
                    <div className="flex items-center gap-3">
                        <div>
                            <h1 className="text-sm font-semibold tracking-widest uppercase text-zinc-200">
                                {currentUsername}
                            </h1>
                            <p className="text-[10px] text-zinc-500">Username</p>
                        </div>
                    </div>

                    <div className="flex items-center gap-6 text-xs border-t sm:border-t-0 border-zinc-800 pt-3 sm:pt-0 w-full sm:w-auto justify-between sm:justify-end">
                        <div
                            ref={notificationRef}
                            onClick={() => setIsNotificationOpen(!isNotificationOpen)}
                            className="cursor-pointer hover:opacity-80 transition-opacity select-none relative"
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
                                                        setSelectedArgumentId(Number(notif.argument_id));
                                                        setTargetCommentId(notif.comment_id || null);
                                                        setIsReviewOpen(true);
                                                        setIsNotificationOpen(false);

                                                        /*if (notif.comment_id) {
                                                            setTimeout(() => {
                                                                const commentEl = document.getElementById(`comment-${notif.comment_id}`);
                                                                if (commentEl) {
                                                                    commentEl.classList.add('bg-emerald-900/40', 'transition-colors', 'duration-500');
                                                                    setTimeout(() => {
                                                                        commentEl.classList.remove('bg-emerald-900/40');
                                                                    }, 1500);
                                                                }
                                                            }, 100);
                                                        }*/
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
                        
                        <div>
                            <span className="text-zinc-500 block text-[10px]">LOGIC SCORE</span>
                            <span className="text-emerald-400 font-bold">{logicScore}</span>
                        </div>
                    </div>
                </header>
                
                <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                    <div className="lg:col-span-2 flex flex-col gap-4">
                        <div className="flex justify-between items-center bg-zinc-950 border border-zinc-800 rounded-lg p-3 text-xs text-zinc-400">
                            <span className="text-zinc-200 font-bold tracking-wider px-1">ACTIVE DEBATES</span>
                            <div className="flex gap-10 bg-zinc-900/60 p-1 rounded-lg border border-zinc-800">
                                <button className="text-emerald-400 hover:underline">Recent</button>
                                <button className="hover:text-zinc-200">Top</button>
                                <button className="text-zinc-400 hover:text-zinc-200">WatchList</button>
                            </div>
                        </div>

                        {(argumentsList || []).map((check) => (
                            <div key={check.id} className="bg-zinc-950 border border-zinc-800 rounded-lg p-5 flex flex-col gap-4 hover:border-zinc-700 transition-colors">
                                <div className="flex justify-between items-center text-[10px] text-zinc-500">
                                    <span>AUTHOR: [{check.author}]</span>
                                    <span className="bg-zinc-900 px-2 py-0.5 rounded border border-zinc-800 text-zinc-300">Pull Request #104</span>
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
                                            setSelectedArgumentId(check.id);
                                            setTargetCommentId(null);
                                            setIsReviewOpen(true);
                                        }}>
                                        Review Argument
                                    </button>
                                </div>
                            </div>
                        ))}
                    </div>

                    <div className="flex flex-col gap-6">
                        {isLoggedIn && (
                            <div className="bg-zinc-950 border border-zinc-800 rounded-lg p-5 flex flex-col gap-4">
                                <h2 className="text-xs font-bold tracking-wider uppercase text-zinc-200">
                                    SUBMIT FOR REVIEW
                                </h2>
                                <p className="text-xs text-zinc-500">
                                    Submit a code proposal, paper, or architectural argument to the blind review pool.
                                </p>
                                <button className="w-full bg-emerald-600 hover:bg-emerald-500 text-black font-semibold py-2 rounded text-xs transition-colors"
                                    onClick={() => setIsCreateOpen(true)}>
                                    Create an Argument
                                </button>
                            </div>
                        )}

                        <div className="bg-zinc-950 border border-zinc-800 rounded-lg p-5 flex flex-col gap-3">
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
                    </div>
                </div>

                <ReviewModal
                    isReviewOpen={isReviewOpen}
                    selectedArgument={selectedArgument}
                    targetCommentId={targetCommentId}
                    setIsReviewOpen={setIsReviewOpen}
                    handleAddReply={handleAddReply}
                />

                {isLoggedIn && (
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
        </main>
    );
}