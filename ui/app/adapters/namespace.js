import ApplicationAdapter from './application';
import codesForError from '../utils/codes-for-error';

export default class NamespaceAdapter extends ApplicationAdapter {
  findRecord(store, modelClass, id) {
    return super.findRecord(...arguments).catch(error => {
      const errorCodes = codesForError(error);
      if (errorCodes.includes('501')) {
        return { Name: id };
      }
    });
  }
}
