import ApplicationSerializer from './application';

export default class ServiceSerializer extends ApplicationSerializer {
  attrs = {
    connect: 'Connect',
  };

  normalize(typeHash, hash) {
    if (!hash.Tags) {
      hash.Tags = [];
    }

    return super.normalize(typeHash, hash);
  }
}
