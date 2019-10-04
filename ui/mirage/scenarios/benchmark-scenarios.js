export function uxrTask0(server) {
  server.createList('agent', 3);
  server.createList('node', 5);
  server.create('job', { ratio: '10: R 1' });
  server.create('job', { ratio: '5: R 1 C 4' });
  server.create('job', { ratio: '10: R 1 F 5 L 1' });
  server.create('job', { ratio: '8: Q 3 S 5' });
  server.create('job', { ratio: '10: C 1' });
}
