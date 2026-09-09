import { useState, useEffect } from 'react';
import { Comment } from './types';

interface CommentFuncProps {
    processedComment: Comment;
    argumentId: string | number;
    onAddReply: (commentId: string, savedComment: { id: number; content: string; created_at: string; user_id?: number; author?: string }) => void;
}

export function CommentFunc({ processedComment, argumentId, onAddReply}: CommentFuncProps) {
    const [isReplying, setIsReplying] = useState(false);
    const [replyText, setReplyText] = useState("");
    const [isLoggedIn, setIsLoggedIn] = useState(false);
    const [currentUserId, setCurrentUserId] = useState<number | null>(null);
    const [currentUsername, setCurrentUsername] = useState<string>("");
    const [isCollapsed, setIsCollapsed] = useState(true);
    
    const [fireCount, setFireCount] = useState(processedComment.fire_count || processedComment.score || 0); 

    useEffect(() => {
        setFireCount(processedComment.fire_count || processedComment.score || 0);
    }, [processedComment.fire_count, processedComment.score]);

    useEffect(() => {
        const token = localStorage.getItem('user_token_swing');
        if (token && token !== 'null' && token !== 'undefined') {
            setIsLoggedIn(true);
            try {
                const payload = JSON.parse(atob(token.split('.')[1]));
                setCurrentUserId(payload.user_id || payload.id);
                setCurrentUsername(payload.username || payload.author || "");
            } catch (err) {
                console.error("Failed to parse user token:", err);
            }
        }
    }, []);

    const handleFireClick = async () => {
        if (!processedComment.id || processedComment.id === "root-id") return;

        const token = localStorage.getItem('user_token_swing');
        if (!token || token === 'null' || token === 'undefined') return;

        const port = process.env.NEXT_PUBLIC_PORT || '2026';

        try {
            const res = await fetch(`http://localhost:${port}/api/comments/fire`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ 
                    comment_id: Number(processedComment.id) 
                })
            });

            if (!res.ok) {
                const errText = await res.text();
                console.error("Fire reaction error -> Status:", res.status, "Message:", errText);
                return;
            }

            const data = await res.json();
            
            if (data && data.fire_count !== undefined) {
                setFireCount(data.fire_count);
            }
        } catch (err) {
            console.error("Error sending fire reaction:", err);
        }
    };

    const isLongText = processedComment.content.length > 150;
    const [isExpandedLong, setIsExpandedLong] = useState(false);
    
    const commentUserId = (processedComment as any).user_id ?? (processedComment as any).userId;
    const isOwnComment = currentUserId !== null && commentUserId !== undefined && Number(currentUserId) === Number(commentUserId);

    return (
        <div className="flex flex-col gap-2 my-2 text-xs">
            <div className="bg-zinc-900 border border-zinc-800 p-3 rounded">
                <div className="flex justify-between items-center text-[10px] text-zinc-500 mb-1">
                    <span className="font-mono text-emerald-400">[{processedComment.author}]</span>
                    <div className="flex gap-3 items-center">
                        <span>{processedComment.timestamp}</span>
                        {processedComment.replies && processedComment.replies.length > 0 && (
                            <button
                                onClick={() => setIsCollapsed(!isCollapsed)} className="text-zinc-400 hover:text-white underline">
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
                        onClick={() => setIsExpandedLong(!isExpandedLong)} className="text-[10px] text-blue-400 hover:underline mt-1 block">
                        {isExpandedLong ? "Show less" : "Read more"}
                    </button>
                )}

                <div className="flex items-center gap-4 mt-2">
                    {isLoggedIn && !isOwnComment && (
                        <button
                            onClick={handleFireClick}
                            className="text-[10px] flex items-center gap-1 text-zinc-400 hover:text-orange-400 transition-colors">
                            🔥 {fireCount} Fire
                        </button>
                    )}

                    {isLoggedIn && (
                        <button
                            onClick={() => setIsReplying(!isReplying)} className="text-[10px] text-zinc-400 hover:text-emerald-400 block">
                            {isReplying ? "Cancel" : "[+ Reply]"}
                        </button>
                    )}
                </div>

                {isReplying && isLoggedIn && (
                    <div className="mt-3 pt-3 border-t border-zinc-800 flex flex-col gap-2">
                        <textarea
                            value={replyText}
                            onChange={(e) => {setReplyText(e.target.value); setIsCollapsed(false);}}
                            placeholder={`Replying to ${processedComment.author}...`}
                            className="w-full bg-zinc-950 border border-zinc-800 p-2 rounded text-xs text-zinc-200 outline-none focus:border-zinc-600" 
                            rows={2}
                        />

                        <button
                            onClick={async () => {
                                if (!replyText.trim()) return;
                                const token = localStorage.getItem('user_token_swing');
                                
                                try {
                                    const res = await fetch('http://localhost:2026/api/comments', {
                                        method: 'POST',
                                        headers: {
                                            'Content-Type': 'application/json',
                                            'Authorization': `Bearer ${token}`
                                        },
                                        body: JSON.stringify({
                                            argument_id: Number(argumentId), 
                                            parent_id: processedComment.id === "root-id" ? null : parseInt(String(processedComment.id)),
                                            content: replyText
                                        }),
                                    });

                                    if (!res.ok) {
                                        throw new Error("Failed to save comment");
                                    }

                                    const savedData = await res.json();

                                    onAddReply(String(processedComment.id), {
                                        id: savedData.id,
                                        content: replyText,
                                        created_at: savedData.created_at,
                                        user_id: currentUserId ?? undefined,
                                        author: currentUsername || savedData.author
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
                    {processedComment.replies.map((check) => (
                        <CommentFunc key={check.id} processedComment={check} argumentId={argumentId} onAddReply={onAddReply} />
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
    const [currentUserId, setCurrentUserId] = useState<number | null>(null);
    const [currentUsername, setCurrentUsername] = useState<string>("");

    useEffect(() => {
        const token = localStorage.getItem('user_token_swing');
        if (token) {
            setIsLoggedIn(true);
            try {
                const payload = JSON.parse(atob(token.split('.')[1]));
                setCurrentUserId(payload.user_id || payload.id);
                setCurrentUsername(payload.username || payload.author || "");
            } catch (err) {
                console.error("Failed to parse user token:", err);
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

    return (
        <div className="flex flex-col gap-2">
            <textarea 
                value={primaryText}
                onChange={(e) => setPrimaryText(e.target.value)}
                placeholder="Write your feedback..." 
                className="w-full bg-zinc-950 border border-zinc-800 p-2 rounded text-xs text-zinc-200 outline-none focus:border-zinc-600" 
                rows={2}
            />

            <button
                onClick={async () => {
                    if (!primaryText.trim()) return;

                    const token = localStorage.getItem('user_token_swing');

                    try {
                        const res =     await fetch('http://localhost:2026/api/comments', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                                'Authorization': `Bearer ${token}`
                            },
                            body: JSON.stringify({
                                argument_id: Number(argumentId),
                                parent_id: null,
                                content: primaryText
                            }),
                        });

                        if (!res.ok) {
                            const errorText = await res.text();
                            console.error("Backend error details:", errorText);
                            throw new Error(`Server error (${res.status}): ${errorText}`);      
                        }
                        const savedData = await res.json();

                        onAddReply("root-id", {
                            id: savedData.id,
                            content: primaryText,
                            created_at: savedData.created_at,
                            user_id: currentUserId ?? undefined,
                            author: currentUsername || savedData.author
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