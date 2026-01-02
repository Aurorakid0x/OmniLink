import request from '../utils/request'
import store from '../store'

// 辅助函数：处理头像/文件 URL
export const normalizeUrl = (url) => {
  if (!url) return ''
  if (url.startsWith('http') || url.startsWith('blob:')) return url
  const backendUrl = store.state.backendUrl || import.meta.env.VITE_BACKEND_URL || 'http://localhost:8000'
  // 确保 backendUrl 不以 / 结尾，url 必须以 / 开头（如果后端返回相对路径）
  // 假设后端返回如 /static/avatars/xxx.png
  return `${backendUrl.replace(/\/$/, '')}${url.startsWith('/') ? '' : '/'}${url}`
}

// 会话相关
export const getSessionList = (ownerId) => {
    // 聚合用户会话和群聊会话，实际项目中可能需要分别调用或合并
    // 这里先提供分别调用的接口
    return Promise.all([
        getUserSessionList(ownerId),
        getGroupSessionList(ownerId)
    ])
}

export const getUserSessionList = (owner_id) => {
  return request.post('/session/getUserSessionList', { owner_id })
}

export const getGroupSessionList = (owner_id) => {
  return request.post('/session/getGroupSessionList', { owner_id })
}

export const openSession = (data) => {
  // data: { send_id, receive_id }
  return request.post('/session/openSession', data)
}

export const deleteSession = (data) => {
  return request.post('/session/deleteSession', data)
}

export const checkOpenSessionAllowed = (data) => {
  // data: { send_id, receive_id }
  return request.post('/session/checkOpenSessionAllowed', data)
}

// 联系人/群相关
export const getUserList = (owner_id) => {
  return request.post('/contact/getUserList', { owner_id })
}

export const loadMyJoinedGroup = (owner_id) => {
  return request.post('/contact/loadMyJoinedGroup', { owner_id })
}

export const getContactInfo = (contact_id) => {
  return request.post('/contact/getContactInfo', { contact_id })
}

export const applyContact = (data) => {
    return request.post('/contact/applyContact', data)
}

export const passContactApply = (data) => {
    return request.post('/contact/passContactApply', data)
}

export const refuseContactApply = (data) => {
    return request.post('/contact/refuseContactApply', data)
}

export const deleteContact = (data) => {
    return request.post('/contact/deleteContact', data)
}

export const blackContact = (data) => {
    return request.post('/contact/blackContact', data)
}

export const getNewContactList = (owner_id) => {
    return request.post('/contact/getNewContactList', { owner_id })
}

// 群组相关
export const createGroup = (data) => {
    return request.post('/group/createGroup', data)
}

export const loadMyGroup = (data) => {
    return request.post('/group/loadMyGroup', data)
}

export const getGroupInfo = (data) => {
    return request.post('/group/getGroupInfo', data)
}

export const getGroupMemberList = (data) => {
    return request.post('/group/getGroupMemberList', data)
}

export const inviteGroupMembers = (data) => {
    // data: { owner_id, group_id, member_ids: [] }
    return request.post('/group/inviteGroupMembers', data)
}

export const leaveGroup = (data) => {
    // data: { owner_id, group_id }
    return request.post('/group/leaveGroup', data)
}

export const dismissGroup = (data) => {
    // data: { owner_id, group_id }
    return request.post('/group/dismissGroup', data)
}

export const removeGroupMembers = (data) => {
    // data: { owner_id, group_id, member_ids: [] }
    return request.post('/group/removeGroupMembers', data)
}

// 消息相关
export const getMessageList = (data) => {
  // data: { user_one_id, user_two_id }
  return request.post('/message/getMessageList', data)
}

export const getGroupMessageList = (data) => {
  // data: { group_id }
  return request.post('/message/getGroupMessageList', data)
}

export const uploadFile = (formData) => {
  return request.post('/message/uploadFile', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

// 移除了未使用的 uploadAvatar 导入和导出，避免混乱
