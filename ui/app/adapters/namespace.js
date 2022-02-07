import Watchable from './watchable';
import codesForError from '../utils/codes-for-error';

export default class NamespaceAdapter extends Watchable {
  findRecord(store, modelClass, id) {
    return super.findRecord(...arguments).catch((error) => {
      const errorCodes = codesForError(error);
      if (errorCodes.includes('501')) {
        return { Name: id };
      }
    });
  }
}
