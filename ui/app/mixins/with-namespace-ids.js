import { inject as service } from '@ember/service';
import Mixin from '@ember/object/mixin';

export default Mixin.create({
  system: service(),

  findAll() {
    const namespace = this.get('system.activeNamespace');
    return this._super(...arguments).then(data => {
      data.forEach(record => {
        record.Namespace = namespace ? namespace.get('id') : 'default';
      });
      return data;
    });
  },

  findRecord(store, type, id, snapshot) {
    const [, namespace] = JSON.parse(id);
    const namespaceQuery = namespace && namespace !== 'default' ? { namespace } : {};

    return this._super(store, type, id, snapshot, namespaceQuery);
  },

  urlForFindAll() {
    const url = this._super(...arguments);
    const namespace = this.get('system.activeNamespace.id');
    return associateNamespace(url, namespace);
  },

  urlForQuery() {
    const url = this._super(...arguments);
    const namespace = this.get('system.activeNamespace.id');
    return associateNamespace(url, namespace);
  },

  urlForFindRecord(id, type, hash) {
    const [name, namespace] = JSON.parse(id);
    let url = this._super(name, type, hash);
    return associateNamespace(url, namespace);
  },

  urlForUpdateRecord(id, type, hash) {
    const [name, namespace] = JSON.parse(id);
    let url = this._super(name, type, hash);
    return associateNamespace(url, namespace);
  },

  xhrKey(url, method, options = {}) {
    const plainKey = this._super(...arguments);
    const namespace = options.data && options.data.namespace;
    return associateNamespace(plainKey, namespace);
  },
});

function associateNamespace(url, namespace) {
  if (namespace && namespace !== 'default') {
    url += `?namespace=${namespace}`;
  }
  return url;
}
