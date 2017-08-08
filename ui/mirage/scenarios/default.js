export default function(server) {
  server.createList('job', 25);
  server.createList('node', 50);
  server.createList('agent', 3);
  server.createList('allocation', 20);
}
