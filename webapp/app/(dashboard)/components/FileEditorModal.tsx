
'use client';

import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';

interface FileEditorModalProps {
  filePath: string;
  initialContent: string;
  onClose: () => void;
  onSave: (filePath: string, content: string) => Promise<void>;
}

export default function FileEditorModal({
  filePath,
  initialContent,
  onClose,
  onSave,
}: FileEditorModalProps) {
  const [content, setContent] = useState(initialContent);
  const [isEditing, setIsEditing] = useState(false);
  const [isMounted, setIsMounted] = useState(false);

  useEffect(() => {
    setIsMounted(true);
  }, []);

  const handleSave = async () => {
    await onSave(filePath, content);
    setIsEditing(false);
  };

  if (!isMounted) return null;

  return createPortal(
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-[100000]">
      <div className="card-vapor neon-border text-white rounded-2xl shadow-2xl w-[min(1100px,94vw)] h-[80vh] flex flex-col border-2 border-neon-purple/30">
        <div className="p-4 border-b border-neon-purple/30 flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-accent-purple/70">File Editor</div>
            <div className="text-sm font-bold text-accent-cyan truncate max-w-[70vw]">{filePath}</div>
          </div>
          <div className="flex items-center gap-2">
            {!isEditing && (
              <button onClick={() => setIsEditing(true)} className="btn-neon px-3 py-1 text-xs font-bold">
                Edit
              </button>
            )}
            {isEditing && (
              <button onClick={handleSave} className="btn-neon px-3 py-1 text-xs font-bold">
                Save
              </button>
            )}
            <button onClick={onClose} className="btn-neon px-3 py-1 text-xs font-bold">
              Close
            </button>
          </div>
        </div>
        <div className="flex-1 p-4 overflow-hidden">
          {isEditing ? (
            <textarea
              className="w-full h-full input-vapor font-mono text-sm resize-none overflow-auto"
              value={content}
              onChange={(e) => setContent(e.target.value)}
            />
          ) : (
            <div className="h-full overflow-auto">
              <pre className="whitespace-pre-wrap text-sm text-accent-cyan/80">{content}</pre>
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body
  );
}
