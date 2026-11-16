// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
#include <stddef.h>
int neroXPCStart(unsigned uid,const char *clientRequirement,char **error);
void neroXPCStop(void);
void *neroXPCRequest(const char *serverRequirement,const char *command,const void *payload,size_t payloadLength,int *status,size_t *length,char **error);
