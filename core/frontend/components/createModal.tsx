import React from 'react';

interface CreateArgumentModalProps {
    isCreateOpen: boolean;
    error: string;
    newTitle: string;
    newContent: string;
    setNewTitle: (newTitleValue: string) => void;
    setNewContent: (newContentValue: string) => void;
    setIsCreateOpen: (isOpen: boolean) => void;
    onSubmit: (formSubmitEvent: React.SyntheticEvent) => void;
}

export function CreateArgumentModal({
    isCreateOpen,
    error,
    newTitle,
    newContent,
    setNewTitle,
    setNewContent,
    setIsCreateOpen,
    onSubmit
}: CreateArgumentModalProps) {
    if (!isCreateOpen) return null;

    return (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex justify-end">
            <div className="bg-zinc-900 border-l border-zinc-700 p-6 text-white w-full max-w-xl h-full overflow-y-auto shadow-xl flex flex-col gap-4">
                <div className="flex justify-between items-center mb-2">
                    <span className="text-xs uppercase font-bold tracking-wider text-emerald-400">New Argument</span>
                    <button onClick={() => setIsCreateOpen(false)} className="text-zinc-400 hover:text-white">✕</button>
                </div>
                
                {error && <p className="text-red-400 text-xs">{error}</p>}

                <form onSubmit={onSubmit} className="flex flex-col gap-4">
                    <div className="flex flex-col gap-1.5">
                        <label className="text-[10px] text-zinc-400 uppercase">Argument Title</label>
                        <input 
                            type="text" 
                            required
                            value={newTitle} 
                            onChange={(changeEvent) => setNewTitle(changeEvent.target.value)}
                            placeholder="The core idea" 
                            className="bg-zinc-950 border border-zinc-800 p-2 rounded text-xs text-zinc-200 outline-none focus:border-zinc-600"
                        />
                    </div>

                    <div className="flex flex-col gap-1.5">
                        <label className="text-[10px] text-zinc-400 uppercase">Content / Premise</label>
                        <textarea 
                            required
                            rows={6}
                            value={newContent} 
                            onChange={(changeEvent) => setNewContent(changeEvent.target.value)}
                            placeholder="Show the logic" 
                            className="bg-zinc-950 border border-zinc-800 p-2 rounded text-xs text-zinc-200 outline-none focus:border-zinc-600 leading-relaxed"
                        />
                    </div>

                    <button 
                        type="submit" 
                        className="bg-emerald-600 hover:bg-emerald-500 text-black font-semibold py-2 rounded text-xs transition-colors mt-2">
                        Submit
                    </button>
                </form>
            </div>
        </div>
    );
}