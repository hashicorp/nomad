import ApplicationSerializer from './application';

export default class NodeEventSerializer extends ApplicationSerializer {
  attrs = {
    time: 'Timestamp',
  };
}
