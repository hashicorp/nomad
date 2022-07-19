import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class ServiceFragmentSerializer extends ApplicationSerializer {
  attrs = {
    connect: 'Connect',
  };

  arrayNullOverrides = ['Tags'];
}
