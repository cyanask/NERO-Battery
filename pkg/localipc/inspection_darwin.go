// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package localipc

/*
#cgo LDFLAGS: -lproc
#include <libproc.h>
#include <sys/proc_info.h>
#include <sys/socket.h>
#include <unistd.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>

// Inspect only executable names/paths, retaining no process inventory.
static int neroAlDenteRunning(void) {
 int count = proc_listallpids(NULL, 0);
 if (count <= 0 || count > 1000000) return -1;
 int capacity = count + 1024;
 pid_t *pids = calloc(capacity, sizeof(pid_t));
 if (!pids) return -1;
 count = proc_listallpids(pids, capacity * sizeof(pid_t));
 if (count < 0 || count >= capacity) { free(pids); return -1; }
 int found = 0;
 for (int i = 0; i < count; i++) {
  if (pids[i] <= 0) continue;
  char path[PROC_PIDPATHINFO_MAXSIZE] = {0};
  if (proc_pidpath(pids[i], path, sizeof(path)) <= 0) {
   if (proc_name(pids[i], path, sizeof(path)) <= 0) continue;
  }
  if (strcasestr(path, "aldente")) { found = 1; break; }
 }
 free(pids);
 return found;
}

// This process inspects its own sockets; an unprivileged external lsof cannot
// reliably see the root helper. This is a sample, not a packet-capture claim.
static int neroOwnSockets(int *internet, int *local) {
 int bytes = proc_pidinfo(getpid(), PROC_PIDLISTFDS, 0, NULL, 0);
 if (bytes <= 0 || bytes > 1024*1024) return -1;
 int capacity = bytes + 64*sizeof(struct proc_fdinfo);
 struct proc_fdinfo *fds = calloc(1, capacity);
 if (!fds) return -1;
 bytes = proc_pidinfo(getpid(), PROC_PIDLISTFDS, 0, fds, capacity);
 if (bytes <= 0 || bytes >= capacity || bytes % sizeof(struct proc_fdinfo)) { free(fds); return -1; }
 int result = 0;
 for (unsigned i = 0; i < bytes/sizeof(struct proc_fdinfo); i++) {
  if (fds[i].proc_fdtype != PROX_FDTYPE_SOCKET) continue;
  struct socket_fdinfo info;
  int size = proc_pidfdinfo(getpid(), fds[i].proc_fd, PROC_PIDFDSOCKETINFO, &info, sizeof(info));
  if (size != sizeof(info)) {
   if (size <= 0 && (errno == EBADF || errno == ENOENT)) continue; // closed during sample
   result = -1; break;
  }
  if (info.psi.soi_family == AF_INET || info.psi.soi_family == AF_INET6) (*internet)++;
  if (info.psi.soi_family == AF_UNIX) (*local)++;
 }
 free(fds);
 return result;
}
*/
import "C"
import "errors"

func CheckNoAlDenteProcess() error {
	switch int(C.neroAlDenteRunning()) {
	case 0:
		return nil
	case 1:
		return errors.New("AlDente is still running; charging takeover refused")
	default:
		return errors.New("cannot inspect competing controller processes")
	}
}

type SocketCounts struct {
	Internet int `json:"internet_sockets"`
	Unix     int `json:"unix_sockets"`
}

func OwnSocketCounts() (SocketCounts, error) {
	var internet, local C.int
	if C.neroOwnSockets(&internet, &local) != 0 {
		return SocketCounts{}, errors.New("cannot inspect own socket descriptors")
	}
	return SocketCounts{int(internet), int(local)}, nil
}
