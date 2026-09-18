import React, { useState, useEffect } from 'react';
import { Argument } from './types';
import { PrimaryCommentInput, CommentFunc } from './comments';
import { getValidToken } from './auth';

interface ReviewModalProps {
    isReviewOpen: boolean;
    selectedArgument: Argument | null;
    targetCommentId?: string | number | null;
    setIsReviewOpen: (value: boolean) => void;
    handleAddReply: (
        targetId: string, 
        savedComment: { 
            id: number; 
            content: string; 
            created_at: string; 
            user_id?: number; 
            author?: string; 
        }
    ) => void;
}

export function ReviewModal({ isReviewOpen, selectedArgument, targetCommentId, setIsReviewOpen, handleAddReply }: ReviewModalProps) {
    const [isViewer, setIsViewer] = useState(false);

    useEffect(() => {
        const token = getValidToken();
        if (token && token !== 'null' && token !== 'undefined') {
            try {
                const tokenPayload = JSON.parse(atob(token.split('.')[1]));
                if (tokenPayload.role === 'viewer') {
                    setIsViewer(true);
                }
            } catch (error) {
                console.error("Failed to parse user token for role:", error);
            }
        }
    }, [isReviewOpen]);

    if (!isReviewOpen || !selectedArgument) return null;

    return (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex justify-between">
            <div className="bg-zinc-900 border-r border-zinc-700 p-6 text-white w-full max-w-xl h-full overflow-y-auto shadow-xl">
            </div>

            <div className="bg-zinc-900 border-l border-zinc-700 p-6 text-white w-full max-w-xl h-full overflow-y-auto shadow-xl">
                <div className="flex justify-between items-center mb-4">
                    <span className="text-sm text-zinc-400">Posted by {selectedArgument?.author}</span>
                    <button onClick={() => {
                        setIsReviewOpen(false);
                        }} 
                        className="text-zinc-400 hover:text-white">✕</button>
                </div>
                    
                <h3 className="text-lg font-bold mb-2">{selectedArgument?.title}</h3>
                <p className="text-sm text-zinc-300 mb-4 whitespace-pre-wrap">
                    {selectedArgument?.content}
                </p>
                    
                <div className="border-t border-zinc-800 pt-4 mt-4 flex flex-col gap-2">
                    <h4 className="text-[10px] font-semibold uppercase text-zinc-500 tracking-wider mb-2">Feedbacks and Comments</h4>

                    <div className="mt-2 pt-3 border-t border-zinc-800/60">
                        <PrimaryCommentInput 
                            argumentId={selectedArgument.id} 
                            isViewer={isViewer}
                            onAddReply={handleAddReply} 
                        />
                    </div>
                    
                    {selectedArgument?.comments && selectedArgument.comments.length > 0 ? (
                        selectedArgument.comments.map((processedComment) => (
                            <CommentFunc 
                                key={processedComment.id} 
                                processedComment={processedComment} 
                                argumentId={selectedArgument.id}
                                targetCommentId={targetCommentId}
                                isViewer={isViewer}
                                onAddReply={handleAddReply} 
                            />
                        ))
                    ) : (
                        <p className="text-xs text-center text-zinc-500">No comments yet</p>
                    )}
                </div>
            </div>
        </div>
    );
}