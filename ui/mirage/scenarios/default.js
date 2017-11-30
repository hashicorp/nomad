export default function(server) {
  server.createList('agent', 3);
  server.createList('node', 50);

  server.createList('namespace', 3);

  server.createList('job', 15);

  server.createList('token', 3);
  logTokens(server);
}

/* eslint-disable */
function logTokens(server) {
  console.log('TOKENS:');
  server.db.tokens.forEach(token => {
    console.log(`
Name: ${token.name}
Secret: ${token.secretId}
Accessor: ${token.accessorId}

`);
  });
}
/* eslint-enable */
