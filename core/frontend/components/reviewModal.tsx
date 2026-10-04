import React, { useEffect, useState } from 'react';
import { Argument } from '../types/types';
import { PrimaryCommentInput, CommentFunc } from './comments';
import { getValidToken } from '../lib/auth';
import { api } from '../lib/api';
import { LineChart, Line, ResponsiveContainer, Tooltip, XAxis, YAxis, CartesianGrid, Legend, ReferenceArea } from 'recharts';
import { useRouter, usePathname, useSearchParams } from 'next/navigation';

interface ReviewModalProps {
    isReviewOpen: boolean;
    selectedArgument: Argument | null;
    selectedArgumentId: number | null;
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
    userStats: any;
    fetchUserStats: () => Promise<void>;
}

export function ReviewModal({ isReviewOpen, selectedArgument, selectedArgumentId, targetCommentId, setIsReviewOpen, handleAddReply, userStats, fetchUserStats }: ReviewModalProps) {
    const [stats, setStats] = useState<any>(null);
    const [detailedArgument, setDetailedArgument] = useState<Argument | null>(null);

    const [left, setLeft] = useState<number | 'dataMin'>('dataMin');
    const [right, setRight] = useState<number | 'dataMax'>('dataMax');
    const [refAreaLeft, setRefAreaLeft] = useState<number | null>(null);
    const [refAreaRight, setRefAreaRight] = useState<number | null>(null);  

    const router = useRouter();
    const pathname = usePathname();
    const searchParams = useSearchParams();

    const activeArg = selectedArgument || detailedArgument;

    const handleLocalAddReply = async (targetId: string, savedComment: any) => {
        if (handleAddReply) {
            handleAddReply(targetId, savedComment);
        }
        await fetchStats();
        await fetchUserStats();

        setDetailedArgument(prev => {
            if (!prev) return prev;

            const comments = prev.comments || [];

            if (String(targetId) === "root-id") {
                return {
                    ...prev,
                    comments: [savedComment, ...comments]
                };
            }

            const insertComment = (commentsList: any[]): any[] => {
                return commentsList.map(c => {
                    if (String(c.id) === String(targetId)) {
                        return {
                            ...c,
                            replies: [...(c.replies || []), savedComment]
                        };
                    }
                    if (c.replies && c.replies.length > 0) {
                        return {
                            ...c,
                            replies: insertComment(c.replies)
                        };
                    }
                    return c;
                });
            };

            if (targetId === "root-id") {
                return {
                    ...prev,
                    comments: [savedComment, ...(prev.comments || [])]
                };
            }

            return {
                ...prev,
                comments: insertComment(prev.comments || [])
            };
        });
    };

    const argumentId = selectedArgument?.id || selectedArgumentId || searchParams.get('argumentId');

    const fetchStats = async () => {
        if (!argumentId) {
            setStats(null);
            setDetailedArgument(null);
            return;
        }

        try {
            const [statsRes, argRes] = await Promise.all([
                api.get(`/api/stats?argument_id=${argumentId}`),
                api.get(`/api/arguments/${argumentId}`)
            ]);

            setStats(statsRes.data);

            const fetchedArg = argRes.data;
            if (fetchedArg && fetchedArg.comments) {
                fetchedArg.comments = [...fetchedArg.comments].reverse();
            }

            setDetailedArgument(argRes.data);
        } catch (err) {
            console.error("Failed to fetch argument stats:", err);
            setStats(null);
            setDetailedArgument(selectedArgument);
        }
    };

    useEffect(() => {
        fetchStats();
    }, [argumentId, searchParams]);

    useEffect(() => {
        if (isReviewOpen && targetCommentId && detailedArgument) {
            const timer = setTimeout(() => {
                const element = document.getElementById(`comment-${targetCommentId}`);
                if (element) {
                    element.scrollIntoView({ behavior: 'smooth', block: 'center' });
                    element.classList.add('highlight-flash');

                    setTimeout(() => {
                        element.classList.remove('highlight-flash');
                    }, 3000);
                }
            }, 300);
        }
    }, [isReviewOpen, targetCommentId, detailedArgument]);

    useEffect(() => {
        if (isReviewOpen && argumentId) {
            fetchStats();
        }
    }, [activeArg?.comments?.length]);

    /*useEffect(() => {
        const argumentId = searchParams.get('argumentId');
        if (!selectedArgument && argumentId) {
            api.get(`/api/argument/${argumentId}`)
            .then(res => setDetailedArgument(res.data))
            .catch(err => console.error("Failed to load argument", err))
        } else {
            setDetailedArgument(null);
        }
    }, [selectedArgument, searchParams]);*/

    const handleZoom = () => {
        if (refAreaLeft === null || refAreaRight === null || refAreaLeft === refAreaRight) {
            setRefAreaLeft(null);
            setRefAreaRight(null);
            return;
        }

        let a = refAreaLeft;
        let b = refAreaRight;

        if (a > b) {
            a = refAreaRight;
            b = refAreaLeft;
        }

        setRefAreaLeft(null);
        setRefAreaRight(null);
        setLeft(a);
        setRight(b);
    };

    const handleResetZoom = () => {
        setLeft('dataMin');
        setRight('dataMax');
        setRefAreaLeft(null);
        setRefAreaRight(null);
    };

    const getCombinedChartData = () => {
        if (!stats || !stats.created_at) return [];

        const map = new Map<number, { timestamp: number; time: string; comments: number; fire: number; activity: number }>();

        const createdAtTime = new Date(stats.created_at).getTime();
        const nowTime = Date.now();
        const hourMs = 60 * 60 * 1000;
        
        let currentBucket = Math.floor(createdAtTime / hourMs) * hourMs;
        const latestBucket = Math.floor(nowTime / hourMs) * hourMs;

        while (currentBucket <= latestBucket) {
            const timeStr = new Date(currentBucket).toISOString();
            map.set(currentBucket, { timestamp: currentBucket, time: timeStr, comments: 0, fire: 0, activity: 0 });
            currentBucket += hourMs;
            if (map.size > 168) break;
        }

        if (map.size === 0) {
            const now = Date.now();
            const nowStr = new Date(now).toISOString();
            map.set(now, { timestamp: now, time: nowStr, comments: 0, fire: 0, activity: 0 });
        }

        (stats.comment_volume || []).forEach((item: any) => {
            const ts = new Date(item.time_bucket).getTime();
            if (map.has(ts)) {
                const entry = map.get(ts)!;
                entry.comments = item.count;
                entry.activity += item.count;
            } else {
                map.set(ts, { timestamp: ts, time: item.time_bucket, comments: item.count, fire: 0, activity: item.count });
            }
        });

        (stats.fire_reactions || []).forEach((item: any) => {
            const ts = new Date(item.time_bucket).getTime();
            if (map.has(ts)) {
                const entry = map.get(ts)!;
                entry.fire = item.count;
                entry.activity += item.count * 2;
            } else {
                map.set(ts, { timestamp: ts, time: item.time_bucket, comments: 0, fire: item.count, activity: item.count * 2 });
            }
        });

        return Array.from(map.values()).sort((a, b) => a.timestamp - b.timestamp);
    };

    const handleWheel = (e: React.WheelEvent<HTMLDivElement>) => {
        e.preventDefault();
        if (!chartData || chartData.length === 0) return;

        const minTime = chartData[0].timestamp;
        const maxTime = chartData[chartData.length - 1].timestamp;

        const currentLeft = left === 'dataMin' ? minTime : left;
        const currentRight = right === 'dataMax' ? maxTime : right;

        const range = currentRight - currentLeft;
        const zoomFactor = 0.1;

        if (e.deltaY > 0) {
            const newLeft = Math.max(minTime, currentLeft - range * zoomFactor);
            const newRight = Math.min(maxTime, currentRight + range * zoomFactor);
            setLeft(newLeft);
            setRight(newRight);
        } else {
            const newLeft = currentLeft + range * zoomFactor;
            const newRight = currentRight - range * zoomFactor;
            if (newRight - newLeft > 60 * 1000) { // Minimum 1 minute window
                setLeft(newLeft);
                setRight(newRight);
            }
        }
    };

    const handleClose = () => {
        setIsReviewOpen(false);

        const params = new URLSearchParams(searchParams.toString());

        params.delete('argumentId');
        params.delete('commentId');

        window.location.hash = '';

        const queryStr = params.toString();

        router.replace(queryStr ? `${pathname}?${queryStr}` : pathname);
        //window.history.pushState(null, '', window.location.pathname + window.location.search);
    };

    const chartData = getCombinedChartData();

    if (!isReviewOpen || (!selectedArgument && !searchParams.get('argumentId'))) return null;

    if (!activeArg) {
        return (
            <div className="fixed inset-0 bg-black/65 backdrop-blur-sm z-50 flex items-center justify-center">
                <div className="text-zinc-400 text-sm animate-pulse">Loading argument...</div>
            </div>
        );
    }

    return (
        <div 
            className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex justify-between overflow-hidden"
            onClick={(e) => {
                if (e.target === e.currentTarget) {
                    handleClose();
                }
            }}>
            <div className="bg-zinc-900 border-r border-zinc-700 p-6 text-white w-full max-w-xl h-full overflow-y-auto shadow-xl flex flex-col gap-6">
                <div className="flex justify-between items-center border-b border-zinc-800 pb-3">
                    <h3 className="text-sm font-semibold uppercase tracking-wider text-zinc-400">Argument Analytics Matrix</h3>
                    <span className="text-xs text-zinc-500">ID: {activeArg?.id}</span>
                </div>

                {stats ? (
                    <div className="space-y-6">
                        <div className="grid grid-cols-3 gap-3">
                            <div className="bg-zinc-800/60 p-3 rounded-lg border border-zinc-800 text-center">
                                <span className="text-[10px] text-zinc-400 block uppercase tracking-wider">Age</span>
                                <span className="text-base font-bold text-zinc-100">
                                    {Math.floor(stats.age_in_seconds / 3600)} hrs
                                </span>
                            </div>
                            <div className="bg-zinc-800/60 p-3 rounded-lg border border-zinc-800 text-center">
                                <span className="text-[10px] text-zinc-400 block uppercase tracking-wider">Comments</span>
                                <span className="text-base font-bold text-blue-400">
                                    {(stats.comment_volume || []).reduce((acc: number, curr: { count: number }) => acc + curr.count, 0)}
                                </span>
                            </div>
                            <div className="bg-zinc-800/60 p-3 rounded-lg border border-zinc-800 text-center">
                                <span className="text-[10px] text-zinc-400 block uppercase tracking-wider">Fire Spikes</span>
                                <span className="text-base font-bold text-orange-400">
                                    {(stats.fire_reactions || []).reduce((acc: number, curr: { count: number }) => acc + curr.count, 0)}
                                </span>
                            </div>
                        </div>

                        <div className="bg-zinc-800/40 p-4 rounded-lg border border-zinc-800 space-y-3">
                            <div className="flex justify-between items-center">
                                <div>
                                    <h4 className="text-xs font-semibold text-zinc-300 uppercase tracking-wider">Engagement & Velocity Trend</h4>
                                    <span className="text-[10px] text-zinc-500">Click & drag to zoom</span>
                                </div>
                                <div className="flex items-center gap-2">
                                    {(left !== 'dataMin' || right !== 'dataMax') && (
                                        <button 
                                            onClick={handleResetZoom}
                                            className="text-[10px] bg-zinc-700 hover:bg-zinc-600 text-zinc-200 px-2 py-1 rounded transition"
                                        >
                                            Default View
                                        </button>
                                    )}
                                    <span className="text-[10px] text-zinc-500">Live Feed</span>
                                </div>
                            </div>

                            <div 
                                className="h-64 w-full select-none"
                                onWheel={handleWheel}
                            >
                                {chartData.length > 0 ? (
                                    <ResponsiveContainer width="100%" height="100%">
                                        <LineChart 
                                            data={chartData}
                                            onMouseDown={(e) => e && setRefAreaLeft(e.activeLabel != null ? Number(e.activeLabel) : null)}
                                            onMouseMove={(e) => refAreaLeft !== null && e && setRefAreaRight(e.activeLabel != null ? Number(e.activeLabel) : null)}
                                            onMouseUp={handleZoom}
                                        >
                                            <CartesianGrid strokeDasharray="3 3" stroke="#27272a" vertical={false} />
                                            <XAxis 
                                                dataKey="timestamp" 
                                                type="number"
                                                scale="time"
                                                domain={[left, right]}
                                                allowDataOverflow={true}
                                                stroke="#71717a" 
                                                fontSize={10} 
                                                tickFormatter={(val: number) => new Date(val).toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})} 
                                            />
                                            <YAxis stroke="#71717a" fontSize={10} allowDecimals={false} />
                                            <Tooltip 
                                                contentStyle={{ backgroundColor: '#18181b', borderColor: '#27272a', borderRadius: '8px', fontSize: '12px', color: '#fff' }} 
                                                labelFormatter={(val) => val ? new Date(Number(val)).toLocaleString() : ''}
                                            />
                                            <Legend wrapperStyle={{ fontSize: '11px', paddingTop: '8px' }} />
                                            
                                            <Line type="monotone" dataKey="comments" name="Comments" stroke="#3b82f6" strokeWidth={2} dot={false} activeDot={{ r: 4 }} />
                                            <Line type="monotone" dataKey="fire" name="Fire Spikes" stroke="#f97316" strokeWidth={2} dot={false} activeDot={{ r: 4 }} />
                                            <Line type="monotone" dataKey="activity" name="Momentum" stroke="#10b981" strokeWidth={1.5} strokeDasharray="4 4" dot={false} />

                                            {refAreaLeft !== null && refAreaRight !== null && (
                                                <ReferenceArea x1={refAreaLeft} x2={refAreaRight} strokeOpacity={0.3} fill="#3b82f6" fillOpacity={0.2} />
                                            )}
                                        </LineChart>
                                    </ResponsiveContainer>
                                ) : (
                                    <div className="h-full flex items-center justify-center text-xs text-zinc-500">
                                        Not enough data points to render trend grid yet.
                                    </div>
                                )}
                            </div>
                        </div>

                        <div className="bg-zinc-800/40 p-4 rounded-lg border border-zinc-800 space-y-1">
                            <h4 className="text-xs font-semibold text-zinc-300 uppercase tracking-wider">Argument Genesis</h4>
                            <p className="text-xs text-zinc-400 leading-relaxed">
                                Initialized on {stats?.created_at ? new Date(stats.created_at).toLocaleString() : 'N/A'}.
                            </p>
                        </div>
                    </div>
                ) : (
                    <div className="flex-1 flex items-center justify-center">
                        <p className="text-xs text-zinc-500 animate-pulse">Loading analytics engine...</p>
                    </div>
                )}
            </div>

            {/* Right Column: Details & Comments */}
            <div className="bg-zinc-900 border-l border-zinc-700 p-6 text-white w-full max-w-xl h-full flex flex-col min-h-0 shadow-xl">
                <div className="flex-shrink-0">
                    <div className="flex justify-between items-center mb-4">
                        <span className="text-sm text-zinc-400">Posted by {activeArg?.author}</span>
                        <button onClick={handleClose} className="text-zinc-400 hover:text-white">✕</button>
                    </div>
                        
                    <h3 className="text-lg font-bold mb-2">{activeArg?.title}</h3>
                    <p className="text-sm text-zinc-300 mb-4 whitespace-pre-wrap">
                        {activeArg?.content}
                    </p>
                        
                    <div className="border-t border-zinc-800 pt-4 mt-4">
                        <h4 className="text-[10px] font-semibold uppercase text-zinc-500 tracking-wider mb-2">Feedbacks and Comments</h4>
                        <PrimaryCommentInput 
                            argumentId={activeArg?.id as any}
                            onAddReply={handleLocalAddReply} 
                        />
                    </div>
                </div>

                <div className="flex-1 min-h-0 overflow-y-auto mt-4 pt-3 border-t border-zinc-800/60 space-y-3 pr-1">
                    {activeArg?.comments && activeArg.comments.length > 0 ? (
                        activeArg.comments.map((processedComment) => (
                            <CommentFunc 
                                key={processedComment.id} 
                                processedComment={processedComment} 
                                argumentId={activeArg.id}
                                targetCommentId={targetCommentId}
                                onAddReply={handleLocalAddReply} 
                                onFire={fetchStats}
                                fetchUserStats={fetchUserStats}
                            />
                        ))
                    ) : (
                        <p className="text-xs text-center text-zinc-500 py-4">No comments yet</p>
                    )}
                </div>
            </div>
        </div>
    );
}