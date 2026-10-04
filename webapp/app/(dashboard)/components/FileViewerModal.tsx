
'use client';

interface FileViewerModalProps {
  content: string;
  onClose: () => void;
}

export default function FileViewerModal({ content, onClose }: FileViewerModalProps) {
  return (
    <div className="fixed inset-0 z-[10000] bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-gray-800 text-white rounded-lg shadow-lg w-3/4 h-3/4 flex flex-col">
        <div className="p-4 border-b border-gray-700 flex justify-between items-center">
          <h2 className="text-lg font-bold">File Content</h2>
          <button onClick={onClose} className="text-white">&times;</button>
        </div>
        <div className="flex-1 p-4 overflow-auto">
          <pre className="whitespace-pre-wrap">{content}</pre>
        </div>
      </div>
    </div>
  );
}
