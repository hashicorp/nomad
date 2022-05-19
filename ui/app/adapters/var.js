import ApplicationAdapter from './application';
import { isArray } from '@ember/array';

export default class VarAdapter extends ApplicationAdapter {
  handleResponse(_status, _headers, response) {
    console.log('handling var response', _status, _headers, response);
    const successful = ('' + _status).startsWith(2);
    if (successful) {
      if (response) {
        if (isArray(response)) {
          response.forEach((item) => (item.ID = item.Path));
        } else {
          response.ID = response.Path;
        }
        return response;
      }
    } else {
      throw new Error(response);
    }
  }
  // PUT instead of POST on create;
  // /v1/var instead of /v1/vars on create (urlForFindRecord)
  createRecord(store, type, snapshot) {
    let data = this.serialize(snapshot);
    return this.ajax(
      this.urlForFindRecord(snapshot.id, snapshot.modelName),
      'PUT',
      { data }
    );
  }
}
