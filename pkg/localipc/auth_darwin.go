// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
package localipc

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation -lbsm
#include <Security/Security.h>
#include <Security/CSCommon.h>
#include <CoreFoundation/CoreFoundation.h>
#include <bsm/libbsm.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <errno.h>
#include <stdlib.h>

// acl_get_* returns NULL for both an absent ACL and an inspection error.
// Query ACL presence only after statx succeeds, so errors never become access.
static int battCheckACLProperty(filesec_t security) {
 int present = 0;
 int result = filesec_query_property(security, FILESEC_ACL, &present);
 filesec_free(security);
 return result == 0 ? (present ? 1 : 0) : -1;
}

static int battCheckNoACL(const char *path) {
 filesec_t security = filesec_init();
 if (security == NULL) return -1;
 struct stat st;
 if (lstatx_np(path, &st, security) != 0) { filesec_free(security); return -1; }
 return battCheckACLProperty(security);
}

static int battCheckNoACLFd(int fd) {
 filesec_t security = filesec_init();
 if (security == NULL) return -1;
 struct stat st;
 if (fstatx_np(fd, &st, security) != 0) { filesec_free(security); return -1; }
 return battCheckACLProperty(security);
}

static int battValidateCode(SecCodeRef code, const char *text, int *detail) {
	CFStringRef string = CFStringCreateWithCString(NULL, text, kCFStringEncodingUTF8);
	if (string == NULL) return 4;
	SecRequirementRef requirement = NULL;
	OSStatus status = SecRequirementCreateWithString(string, kSecCSDefaultFlags, &requirement);
	CFRelease(string);
	if (status != errSecSuccess) { *detail = (int)status; return 4; }
	status = SecCodeCheckValidity(code, kSecCSStrictValidate, requirement);
	CFRelease(requirement);
	if (status != errSecSuccess) { *detail = (int)status; return 5; }
	CFDictionaryRef info = NULL;
	status = SecCodeCopySigningInformation(code, kSecCSSigningInformation, &info);
	if (status != errSecSuccess || info == NULL) { *detail = (int)status; return 6; }
	CFTypeRef flagsRef = CFDictionaryGetValue(info, kSecCodeInfoFlags);
	int flags = 0;
	if (flagsRef == NULL || CFGetTypeID(flagsRef) != CFNumberGetTypeID() ||
		!CFNumberGetValue((CFNumberRef)flagsRef, kCFNumberIntType, &flags) ||
		(flags & kSecCodeSignatureRuntime) == 0) {
		CFRelease(info); return 7;
	}
	CFDictionaryRef entitlements = (CFDictionaryRef)CFDictionaryGetValue(info, kSecCodeInfoEntitlementsDict);
	if (entitlements != NULL) {
		if (CFGetTypeID(entitlements) != CFDictionaryGetTypeID()) { CFRelease(info); return 8; }
		const CFStringRef unsafeKeys[] = {
			CFSTR("com.apple.security.get-task-allow"),
			CFSTR("com.apple.security.cs.disable-library-validation"),
			CFSTR("com.apple.security.cs.allow-unsigned-executable-memory"),
			CFSTR("com.apple.security.cs.allow-jit"),
			CFSTR("com.apple.security.cs.disable-executable-page-protection")
		};
		for (unsigned i = 0; i < sizeof(unsafeKeys) / sizeof(unsafeKeys[0]); i++) {
			CFTypeRef value = CFDictionaryGetValue(entitlements, unsafeKeys[i]);
			if (value != NULL && (CFGetTypeID(value) != CFBooleanGetTypeID() || CFBooleanGetValue((CFBooleanRef)value))) {
				CFRelease(info); return 8;
			}
		}
	}
	CFRelease(info);
	return 0;
}

// The kernel supplies the audit token. There is no PID-only or identifier-only fallback.
static int battVerifyPeer(int fd, unsigned expectedUID, const char *text, int *detail) {
	audit_token_t token;
	socklen_t size = sizeof(token);
	if (getsockopt(fd, SOL_LOCAL, LOCAL_PEERTOKEN, &token, &size) != 0) {
		*detail = errno; return 1;
	}
	if (size != sizeof(token)) { *detail = EPROTO; return 1; }
	if (audit_token_to_euid(token) != expectedUID) return 2;
	CFDataRef data = CFDataCreate(NULL, (const UInt8 *)&token, sizeof(token));
	if (data == NULL) return 3;
	const void *keys[] = { kSecGuestAttributeAudit };
	const void *values[] = { data };
	CFDictionaryRef attrs = CFDictionaryCreate(NULL, keys, values, 1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFRelease(data);
	if (attrs == NULL) return 3;
	SecCodeRef code = NULL;
	OSStatus status = SecCodeCopyGuestWithAttributes(NULL, attrs, kSecCSDefaultFlags, &code);
	CFRelease(attrs);
	if (status != errSecSuccess || code == NULL) { *detail = (int)status; return 3; }
	int result = battValidateCode(code, text, detail);
	CFRelease(code);
	return result;
}

static int battVerifySelf(const char *text, int *detail) {
	SecCodeRef code = NULL;
	OSStatus status = SecCodeCopySelf(kSecCSDefaultFlags, &code);
	if (status != errSecSuccess || code == NULL) { *detail = (int)status; return 3; }
	int result = battValidateCode(code, text, detail);
	CFRelease(code);
	return result;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"net"
	"unsafe"
)

// VerificationError contains a stage and OS status, not caller identity data.
type VerificationError struct {
	Stage  int
	Status int
}

func checkNoExtendedACL(path string) error {
	name := C.CString(path)
	defer C.free(unsafe.Pointer(name))
	if C.battCheckNoACL(name) != 0 {
		return errors.New("protected IPC paths must have no extended ACL; refusing unreadable ACLs as well")
	}
	return nil
}

func checkNoExtendedACLFD(fd int) error {
	if C.battCheckNoACLFd(C.int(fd)) != 0 {
		return errors.New("IPC policy descriptor has an extended or unreadable ACL")
	}
	return nil
}

func (e *VerificationError) Error() string {
	return fmt.Sprintf("local IPC identity rejected (stage %d, status %d)", e.Stage, e.Status)
}

func (p *Policy) VerifySelf() error {
	if p == nil || p.serverRequirement == "" {
		return errors.New("a validated signing policy is required")
	}
	requirement := C.CString(p.serverRequirement)
	defer C.free(unsafe.Pointer(requirement))
	var detail C.int
	if stage := C.battVerifySelf(requirement, &detail); stage != 0 {
		return &VerificationError{Stage: int(stage), Status: int(detail)}
	}
	return nil
}

func (p *Policy) VerifyPeer(conn *net.UnixConn) error {
	if p == nil || p.clientRequirement == "" || conn == nil {
		return errors.New("a validated signing policy and Unix connection are required")
	}
	return verifyConnection(conn, p.uid, p.clientRequirement)
}

// VerifyServer mutually authenticates the privileged endpoint, including UID 0.
func (p *Policy) VerifyServer(conn *net.UnixConn) error {
	if p == nil || p.serverRequirement == "" || conn == nil {
		return errors.New("validated server policy and Unix connection required")
	}
	return verifyConnection(conn, 0, p.serverRequirement)
}

func verifyConnection(conn *net.UnixConn, uid uint32, requirementText string) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	requirement := C.CString(requirementText)
	defer C.free(unsafe.Pointer(requirement))
	var stage, detail C.int
	if err := raw.Control(func(fd uintptr) {
		stage = C.battVerifyPeer(C.int(fd), C.uint(uid), requirement, &detail)
	}); err != nil {
		return err
	}
	if stage != 0 {
		return &VerificationError{Stage: int(stage), Status: int(detail)}
	}
	return nil
}

func (p *Policy) VerifyXPCServer() error {
	if err := checkServerExecutable(); err != nil {
		return err
	}
	return p.VerifySelf()
}
