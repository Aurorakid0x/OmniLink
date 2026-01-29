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
 * Get system AI assistant session
 * @returns {Promise}
 */
export const getSystemSession = () => {
  return request.get('/ai/assistant/system-session')
}

/**
 * Get user's AI assistant sessions (support filtering by type)
 * @param {Object} params - { limit, offset, type }
 * @returns {Promise}
 */
export const getSessions = (params = {}) => {
  return request.get('/ai/assistant/sessions', { params })
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

/**
 * Create a new AI Agent
 * @param {Object} data - { name, description, persona_prompt, kb_type, kb_name }
 * @returns {Promise}
 */
export const createAgent = (data) => {
  return request.post('/ai/assistant/agents', data)
}

/**
 * Create a new Session
 * @param {Object} data - { agent_id, title }
 * @returns {Promise}
 */
export const createSession = (data) => {
  return request.post('/ai/assistant/sessions', data)
}
