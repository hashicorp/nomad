import ApplicationAdapter from './application';
import { isArray } from '@ember/array';
import { pluralize } from 'ember-inflector';
import classic from 'ember-classic-decorator';

@classic
export default class VariableAdapter extends ApplicationAdapter {
  pathForType() {
    return 'var';
  }
  handleResponse(_status, _headers, response) {
    const successful = ('' + _status).startsWith(2);
    if (successful) {
      if (response) {
        if (isArray(response)) {
          response.forEach((item) => (item.ID = item.Path));
          // TODO: do this in the serializer. Map path to id.
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
  createRecord(_store, _type, snapshot) {
    let data = this.serialize(snapshot);
    return this.ajax(
      this.urlForFindRecord(snapshot.id, snapshot.modelName),
      'PUT',
      { data }
    );
  }

  urlForFindAll(modelName) {
    let baseUrl = this.buildURL(modelName);
    return pluralize(baseUrl);
  }
  urlForFindRecord(id, modelName, snapshot) {
    let baseUrl = this.buildURL(modelName, id, snapshot);
    return baseUrl;
  }
}
