import ApplicationSerializer from './application';

export default class NodeAttributes extends ApplicationSerializer {
  normalize(typeHash, hash) {
    return super.normalize(typeHash, { NodeAttributes: hash });
  }
}
