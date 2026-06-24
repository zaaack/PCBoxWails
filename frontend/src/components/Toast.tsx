import React, { useEffect, useState } from 'react';
import { useStore } from '../store';
import { FiX, FiCheck, FiInfo } from 'react-icons/fi';

export const GlobalToast: React.FC = () => {
  const { toasts, removeToast } = useStore();

  if (toasts.length === 0) return null;

  return (
    <div className="global-toast-container">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} onRemove={removeToast} />
      ))}
    </div>
  );
};

interface ToastItemProps {
  toast: {
    id: string;
    message: string;
    type: 'success' | 'error' | 'info';
    action?: { label: string; onClick: () => void };
    duration?: number;
  };
  onRemove: (id: string) => void;
}

const ToastItem: React.FC<ToastItemProps> = ({ toast, onRemove }) => {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    requestAnimationFrame(() => setVisible(true));
    const timer = setTimeout(() => {
      setVisible(false);
      setTimeout(() => onRemove(toast.id), 300);
    }, toast.duration || 5000);
    return () => clearTimeout(timer);
  }, []);

  return (
    <div className={`global-toast global-toast-${toast.type} ${visible ? 'show' : 'hide'}`}>
      <span className="global-toast-icon">
        {toast.type === 'success' && <FiCheck size={14} />}
        {toast.type === 'error' && <FiX size={14} />}
        {toast.type === 'info' && <FiInfo size={14} />}
      </span>
      <span className="global-toast-message">{toast.message}</span>
      <div className="global-toast-actions">
        {toast.action && (
          <button
            className="btn btn-xs btn-toast-action"
            onClick={toast.action.onClick}
          >
            {toast.action.label}
          </button>
        )}
        <button
          className="btn btn-xs btn-toast-close"
          onClick={() => {
            setVisible(false);
            setTimeout(() => onRemove(toast.id), 300);
          }}
        >
          <FiX size={12} />
        </button>
      </div>
    </div>
  );
};
