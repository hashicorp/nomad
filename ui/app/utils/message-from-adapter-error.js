// Returns a single string based on the response the adapter received
export default function messageFromAdapterError(error) {
  if (error.errors) {
    return error.errors.mapBy('detail').join('\n\n');
  }
}
