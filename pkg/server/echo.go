package server

const maxPings = 30

func (s *Server) handleEcho(c *conn) {
	for i := 0; i < maxPings; i++ {
		b, err := c.reader.ReadByte()
		if err != nil {
			c.logger.Debug("echo read", "err", err)
			return
		}
		if _, err := c.rwc.Write([]byte{b}); err != nil {
			c.logger.Debug("echo write", "err", err)
			return
		}
	}
}
