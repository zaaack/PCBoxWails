import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './styles/index.css';

window.addEventListener('contextmenu', (e) => {
  if (e.ctrlKey) {
    e.preventDefault();
    (window as any).runtime?.OpenDevTools();
  }
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <App />
);
