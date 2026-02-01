export const normalizeSession = (raw) => {
  // raw might come from getUserSessionList (user_id) or getGroupSessionList (group_id)
  // or openSession result
  
  // Try to determine peer_id
  let peerId = raw.peer_id || raw.user_id || raw.group_id || raw.receive_id;
  
  // Try to determine type
  let peerType = raw.peer_type;
  if (!peerType) {
    if (peerId && peerId.startsWith('G')) peerType = 'G';
    else if (peerId && peerId.startsWith('U')) peerType = 'U';
    else if (raw.group_id) peerType = 'G';
    else peerType = 'U'; // Default to User
  }

  // Handle avatar
  let avatar = raw.peer_avatar || raw.avatar || raw.receive_avatar || '';

  // Handle name
  let name = raw.peer_name || raw.username || raw.group_name || raw.receive_name || 'Unknown';

  return {
    session_id: raw.session_id,
    peer_id: peerId,
    peer_type: peerType,
    peer_name: name,
    peer_avatar: avatar,
    updated_at: raw.updated_at || new Date().toISOString(),
    last_msg: raw.last_msg || '',
    unread_count: raw.unread_count || 0, // Initial unread count if available
    // Keep original raw data just in case
    ...raw
  };
};

export const normalizeIncomingMessage = (raw, currentUserId) => {
  // Standardize fields
  const sendId = raw.send_id || raw.sendId || '';
  const receiveId = raw.receive_id || raw.receiveId || '';
  
  // Determine session/peer logic
  let peerId = '';
  if (receiveId && receiveId.startsWith('G')) {
    peerId = receiveId;
  } else {
    // Private chat
    peerId = (sendId === currentUserId) ? receiveId : sendId;
  }

  return {
    uuid: raw.uuid || raw.id || `temp-${Date.now()}`,
    session_id: raw.session_id || raw.sessionId,
    send_id: sendId,
    receive_id: receiveId,
    type: raw.type !== undefined ? raw.type : 0, // 0: Text, 1: Image, 2: File, 3: System/Call
    content: raw.content || '',
    url: raw.url || '',
    file_name: raw.file_name || raw.fileName || '',
    file_size: raw.file_size || raw.fileSize || '',
    file_type: raw.file_type || raw.fileType || '',
    send_avatar: raw.send_avatar || raw.sendAvatar || '',
    send_name: raw.send_name || raw.sendName || '',
    created_at: raw.created_at || raw.createdAt || new Date().toISOString(),
    mentioned_user_ids: raw.mentioned_user_ids || raw.mentionedUserIds || [],
    mention_all: raw.mention_all || raw.mentionAll || false,
    // Calculated field
    peer_id: peerId
  };
};
