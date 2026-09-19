import { useState, useEffect } from 'react';
import { Comment } from './types';
import { api } from './api';
import { getValidToken } from './auth';

interface CommentFuncProps {
    processedComment: Comment;
    argumentId: string | number;
    targetCommentId?: string | number | null;
    onAddReply: (
        commentId: string, 
        savedComment: { 
            id: number;
            content: string; 
            created_at: string; 
            user_id?: number; 
            author?: string 
        }) => void;
}

export function CommentFunc({ processedComment, argumentId, targetCommentId, onAddReply }: CommentFuncProps) {
    const [isReplying, setIsReplying] = useState(false);
    const [replyText, setReplyText] = useState("");
    const [isLoggedIn, setIsLoggedIn] = useState(false);
    const [isViewer, setIsViewer] = useState(false);
    const [currentUserId, setCurrentUserId] = useState<number | null>(null);
    const [currentUsername, setCurrentUsername] = useState<string>("");
    const [isCollapsed, setIsCollapsed] = useState(true);
    
    const [fireCount, setFireCount] = useState(processedComment.fire_count || processedComment.score || 0); 
    const [isHighlighted, setIsHighlighted] = useState(false);

    useEffect(() => {
        if (targetCommentId && String(processedComment.id) === String(targetCommentId)) {
            setIsHighlighted(true);
            const timer = setTimeout(() => {
                setIsHighlighted(false);
            }, 1000);
            return () => clearTimeout(timer);
        }
    }, [targetCommentId, processedComment.id]);

    useEffect(() => {
        setFireCount(processedComment.fire_count || processedComment.score || 0);
    }, [processedComment.fire_count, processedComment.score]);

    useEffect(() => {
        if (targetCommentId && processedComment.replies) {
            const hasTargetInReplies = (replyList: Comment[]): boolean => {
                return replyList.some(replyItem => {
                    const isTargetMatch = String(replyItem.id) === String(targetCommentId);
                    const hasNestedTarget = Boolean(replyItem.replies && hasTargetInReplies(replyItem.replies));

                    return isTargetMatch || hasNestedTarget;
                });
            };
            
            if (String(processedComment.id) === String(targetCommentId) || hasTargetInReplies(processedComment.replies)) {
                setIsCollapsed(false);
            }
        }
    }, [targetCommentId, processedComment]);

    useEffect(() => {
        const sessionRole = sessionStorage.getItem('user_role') || localStorage.getItem('user_role');
        if (sessionRole === 'viewer') {
            setIsViewer(true);
        }

        const token = getValidToken();

        if (token && token !== 'null' && token !== 'undefined') {
            setIsLoggedIn(true);

            try {
                const tokenPayload = JSON.parse(atob(token.split('.')[1]));
                setCurrentUserId(tokenPayload.user_id || tokenPayload.id);
                setCurrentUsername(tokenPayload.username || tokenPayload.author || "");
                
                if (tokenPayload.role === 'viewer') {
                    setIsViewer(true);
                }
            } catch (error) {
                console.error("Failed to parse user token:", error);
            }
        }
    }, []);

    const handleFireClick = async () => {
        if (isViewer || !processedComment.id || processedComment.id === "root-id") return;

        try {
            const response = await api.post(`/api/comments/fire`, { comment_id: Number(processedComment.id) });
            if (response.data && response.data.fire_count !== undefined) setFireCount(response.data.fire_count);
        } catch (error) {
            console.error("Error sending fire reaction:", error);
        }
    };

    const isLongText = processedComment.content.length > 150;
    const [isExpandedLong, setIsExpandedLong] = useState(false);
    
    const commentUserId = (processedComment as any).user_id ?? (processedComment as any).userId;
    const isOwnComment = currentUserId !== null && commentUserId !== undefined && Number(currentUserId) === Number(commentUserId);

    return (
        <div className="flex flex-col gap-2 my-2 text-xs">
            <div
                id={`comment-${processedComment.id}`}
                className={`border p-3 rounded transition-colors duration-500 ${
                    isHighlighted 
                        ? 'bg-emerald-900/60 border-emerald-500' 
                        : 'bg-zinc-900 border-zinc-800'
                }`}
                >
                <div className="flex justify-between items-center text-[10px] text-zinc-500 mb-1">
                    <span className="font-mono text-emerald-400">[{processedComment.author}]</span>
                    <div className="flex gap-3 items-center">
                        <span>{processedComment.timestamp}</span>
                        {processedComment.replies && processedComment.replies.length > 0 && (
                            <button
                                onClick={() => setIsCollapsed(!isCollapsed)} 
                                className="text-zinc-400 hover:text-white underline">
                                {isCollapsed ? `View: ${processedComment.replies.length}` : "Unview"}
                            </button>
                        )}
                    </div>
                </div>

                <p className="text-zinc-300 leading-relaxed break-words whitespace-pre-wrap min-w-0">
                    {isLongText && !isExpandedLong ? `${processedComment.content.substring(0, 150)}...` : processedComment.content}
                </p>

                {isLongText && (
                    <button
                        onClick={() => setIsExpandedLong(!isExpandedLong)} 
                        className="text-[10px] text-blue-400 hover:underline mt-1 block">
                        {isExpandedLong ? "Show less" : "Read more"}
                    </button>
                )}

                <div className="flex items-center gap-4 mt-2">
                    {isLoggedIn && !isViewer && !isOwnComment && (
                        <button
                            onClick={handleFireClick}
                            className="text-[10px] flex items-center gap-1 text-zinc-400 hover:text-orange-400 transition-colors">
                            🔥 {fireCount} Fire
                        </button>
                    )}

                    {isLoggedIn && !isViewer && (
                        <button
                            onClick={() => setIsReplying(!isReplying)} 
                            className="text-[10px] text-zinc-400 hover:text-emerald-400 block">
                            {isReplying ? "Cancel" : "[+ Reply]"}
                        </button>
                    )}
                </div>

                {isReplying && isLoggedIn && !isViewer && (
                    <div className="mt-3 pt-3 border-t border-zinc-800 flex flex-col gap-2">
                        <textarea
                            value={replyText}
                            onChange={(changeEvent) => { setReplyText(changeEvent.target.value); setIsCollapsed(false); }}
                            placeholder={`Replying to ${processedComment.author}...`}
                            className="w-full bg-zinc-950 border border-zinc-800 p-2 rounded text-xs text-zinc-200 outline-none focus:border-zinc-600" 
                            rows={2}
                        />

                        <button
                            onClick={async () => {
                                if (!replyText.trim()) return;
                                
                                try {
                                    await api.post('/api/comments', {
                                            argument_id: Number(argumentId), 
                                            parent_id: processedComment.id === "root-id" ? null : parseInt(String(processedComment.id)),
                                            content: replyText  
                                    });

                                    setReplyText("");
                                    setIsReplying(false);
                                } catch (err) {
                                    console.error("Error posting reply:", err);
                                }
                            }} 
                            className="bg-emerald-600 hover:bg-emerald-500 text-black font-semibold py-1 rounded text-[10px] self-end px-3 transition-colors">
                            Send Reply
                        </button>
                    </div>
                )}
            </div>

            {!isCollapsed && processedComment.replies && processedComment.replies.length > 0 && (
                <div className="ml-4 pl-3 border-l-2 border-zinc-800 flex flex-col gap-2">
                    {processedComment.replies.map((replyItem) => (
                        <CommentFunc 
                            key={replyItem.id} 
                            processedComment={replyItem} 
                            argumentId={argumentId} 
                            targetCommentId={targetCommentId}
                            onAddReply={onAddReply} 
                        />
                    ))}
                </div>
            )}
        </div>
    );
}

interface PrimaryCommentInputProps {
    argumentId: string | number;
    onAddReply: (commentId: string, savedComment: { id: number; content: string; created_at: string; user_id?: number; author?: string }) => void;
}

export function PrimaryCommentInput({ argumentId, onAddReply }: PrimaryCommentInputProps) {
    const [primaryText, setPrimaryText] = useState("");
    const [isLoggedIn, setIsLoggedIn] = useState(false);
    const [isViewer, setIsViewer] = useState(false);
    const [currentUserId, setCurrentUserId] = useState<number | null>(null);
    const [currentUsername, setCurrentUsername] = useState<string>("");

    useEffect(() => {
        const sessionRole = sessionStorage.getItem('user_role') || localStorage.getItem('user_role');
        if (sessionRole === 'viewer') {
            setIsViewer(true);
        }

        const token = getValidToken();

        if (token && token !== 'null' && token !== 'undefined') {
            setIsLoggedIn(true);

            try {
                const tokenPayload = JSON.parse(atob(token.split('.')[1]));
                setCurrentUserId(tokenPayload.user_id || tokenPayload.id);
                setCurrentUsername(tokenPayload.username || tokenPayload.author || "");
                
                if (tokenPayload.role === 'viewer') {
                    setIsViewer(true);
                }
            } catch (error) {
                console.error("Failed to parse user token:", error);
            }
        }
    }, []);

    if (!isLoggedIn) {
        return (
            <div className="text-zinc-500 text-xs italic py-2">
                Log in first to post a comment :)
            </div>
        );
    }

    if (isViewer) {
        return (
            <div className="text-zinc-500 text-xs italic py-2">
                Viewer accounts cannot post comments.
            </div>
        );
    }

    return (
        <div className="flex flex-col gap-2">
            <textarea 
                value={primaryText}
                onChange={(changeEvent) => setPrimaryText(changeEvent.target.value)}
                placeholder="Write your feedback..." 
                className="w-full bg-zinc-950 border border-zinc-800 p-2 rounded text-xs text-zinc-200 outline-none focus:border-zinc-600" 
                rows={2}
            />

            <button
                onClick={async () => {
                    if (!primaryText.trim()) return;

                    try {
                        await api.post('/api/comments', {
                                argument_id: Number(argumentId),
                                parent_id: null,
                                content: primaryText
                        });
                        
                        setPrimaryText("");
                    } catch (err) {
                        console.error("Error posting root comment:", err);
                    }
                }} 
                className="bg-emerald-600 hover:bg-emerald-500 text-black font-semibold py-1 rounded text-[10px] self-end px-3 transition-colors">
                Send Reply
            </button>
        </div>
    );
}