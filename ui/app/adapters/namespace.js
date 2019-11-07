import ApplicationAdapter from './application';
import codesForError from '../utils/codes-for-error';

export default ApplicationAdapter.extend({
  findRecord(store, modelClass, id) {
    return this._super(...arguments).catch(error => {
      const errorCodes = codesForError(error);
      if (errorCodes.includes('501')) {
        return { Name: id };
      }
    });
  },
});
