export default function(server) {
  server.createList('agent', 3);
  server.createList('node', 50);

  server.createList('namespace', 3);

  server.createList('job', 10);
  server.createList('job', 5, { failedPlacements: true });

  server.createList('token', 3);

  // load fixtures from /mirage/fixtures/{argument}.js
  // comment out for now as until we have fixtures covering the entire app properly
  // otherwise it can interfere with the faker data

  // server.loadFixtures('jobs', 'taskGroups', 'allocations', 'tasks', 'taskStates');

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
