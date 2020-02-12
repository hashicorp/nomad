export default function escapeTaskName(taskName) {
  return taskName.replace(/[^a-zA-Z0-9,._+@%/-]/g, '\\$&');
}
