import { useState, useEffect, useRef } from 'react';

function CollaborativeEditor() {
  const [connected, setConnected] = useState(false);
  const [content, setContent] = useState('');
  const [error, setError] = useState(null);
  const wsRef = useRef(null);

  // WebSocket接続を確立
  useEffect(() => {
    const connectWebSocket = () => {
      try {
        // WebSocketのプロトコルを使用（重要）
        wsRef.current = new WebSocket('ws://localhost:8080/ws');

        // 接続開始イベント
        wsRef.current.onopen = () => {
          console.log('WebSocketに接続しました');
          setConnected(true);
          setError(null);
        };

        // メッセージ受信イベント
        wsRef.current.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            console.log('受信データ:', data);

            if (data.type === 'document_state' || data.type === 'content_update') {
              setContent(data.content);
            }
          } catch (err) {
            console.error('メッセージ処理エラー:', err);
          }
        };

        // エラーイベント
        wsRef.current.onerror = (event) => {
          console.error('WebSocketエラー:', event);
          setError('接続エラーが発生しました');
          setConnected(false);
        };

        // 接続終了イベント
        wsRef.current.onclose = () => {
          console.log('WebSocket接続が閉じられました');
          setConnected(false);
          // 数秒後に再接続を試みる
          setTimeout(() => {
            if (!wsRef.current || wsRef.current.readyState === WebSocket.CLOSED) {
              connectWebSocket();
            }
          }, 3000);
        };
      } catch (err) {
        console.error('WebSocket初期化エラー:', err);
        setError('WebSocketの初期化に失敗しました');
        setConnected(false);
      }
    };

    connectWebSocket();

    // クリーンアップ
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  // コンテンツ更新処理
  const updateContent = (newContent) => {
    setContent(newContent);

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      const message = {
        type: 'content_update',
        content: newContent,
        timestamp: Date.now()
      };
      wsRef.current.send(JSON.stringify(message));
    } else {
      console.warn('WebSocket接続がないため、更新を送信できません');
      setError('サーバーとの接続が切断されています');
    }
  };

  return (
    <div className="editor-container">
      <div className="status-bar">
        {connected ? (
          <span className="status-connected">接続済み</span>
        ) : (
          <span className="status-disconnected">未接続</span>
        )}
        {error && <span className="error-message">{error}</span>}
      </div>

      <textarea
        className="editor"
        value={content}
        onChange={(e) => updateContent(e.target.value)}
        placeholder="ここにテキストを入力してください..."
        disabled={!connected}
      />
    </div>
  );
}

export default CollaborativeEditor;
