import request from '../utils/request'

// AI Assistant APIs

/**
 * Send a message to AI assistant (non-streaming)
 * @param {Object} data - { question, session_id, agent_id }
 * @returns {Promise}
 */
export const chat = (data) => {
  return request.post('/ai/assistant/chat', data)
}

/**
 * Send a message to AI assistant with streaming response
 * NOTE: Returns a Response object, not JSON
 * Caller needs to handle SSE parsing manually
 * @param {Object} data - { question, session_id, agent_id }
 * @returns {Promise<Response>}
 */
export const chatStream = async (data) => {
  const token = localStorage.getItem('token')
  const backendUrl = import.meta.env.VITE_BACKEND_URL || 'http://localhost:8000'
  
  const response = await fetch(`${backendUrl}/ai/assistant/chat/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': token ? `Bearer ${token}` : ''
    },
    body: JSON.stringify(data)
  })
  
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`)
  }
  
  return response
}

/**
 * Get user's AI assistant sessions
 * @returns {Promise}
 */
export const getSessions = () => {
  return request.get('/ai/assistant/sessions')
}

/**
 * Get available AI agents
 * @returns {Promise}
 */
export const getAgents = () => {
  return request.get('/ai/assistant/agents')
}

/**
 * Get session message history
 * @param {string} sessionId - Session ID
 * @param {Object} params - { limit, offset }
 * @returns {Promise}
 */
export const getSessionMessages = (sessionId, params = {}) => {
  return request.get(`/ai/assistant/sessions/${sessionId}/messages`, { params })
}
