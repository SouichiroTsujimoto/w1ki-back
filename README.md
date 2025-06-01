# リアルタイム共同編集アプリケーション

## WebSocket接続の設定方法

ReactクライアントからWebSocketに接続するためのコード例：

```javascript
// WebSocket接続の例
const connectWebSocket = () => {
  // wss:// は本番環境用、ws:// は開発環境用
  const ws = new WebSocket('ws://localhost:8080/ws');

  ws.onopen = () => {
    console.log('WebSocketに接続しました');
  };

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('受信データ:', data);
    // 受信したメッセージを処理
  };

  ws.onerror = (error) => {
    console.error('WebSocket接続エラー:', error);
  };

  ws.onclose = () => {
    console.log('WebSocket接続が閉じられました');
    // 再接続ロジックをここに追加することもできます
  };

  return ws;
};

// メッセージ送信の例
const sendMessage = (ws, type, content) => {
  if (ws && ws.readyState === WebSocket.OPEN) {
    const message = {
      type: type,
      content: content,
      timestamp: Date.now()
    };
    ws.send(JSON.stringify(message));
  } else {
    console.error('WebSocketが接続されていません');
  }
};
```

## 注意事項

1. WebSocket接続時にブラウザからの通信がCORSポリシーに準拠していることを確認してください
2. バックエンドでは、`AllowWebSockets: true`が設定されていることを確認
3. 接続に問題がある場合はブラウザのコンソールでエラーメッセージを確認してください
