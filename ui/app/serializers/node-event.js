import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class NodeEventSerializer extends ApplicationSerializer {
  attrs = {
    time: 'Timestamp',
  };
}
