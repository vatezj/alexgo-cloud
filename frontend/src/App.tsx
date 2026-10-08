import { useUsers } from './hooks/useUsers'
import { useState } from 'react'

export default function App() {
  const [token, setToken] = useState('')
  const { data, isLoading, error } = useUsers(token)

  return (
    <div style={{ padding: 16 }}>
      <h1>alexGo-cloud</h1>
      <div>
        <input
          placeholder="Bearer token"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          style={{ width: 480 }}
        />
      </div>
      {isLoading && <div>loading...</div>}
      {error && <div>error</div>}
      <pre style={{ marginTop: 16 }}>{JSON.stringify(data, null, 2)}</pre>
    </div>
  )
}

