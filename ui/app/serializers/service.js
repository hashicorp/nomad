import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class ServiceSerializer extends ApplicationSerializer {
  attrs = {
    connect: 'Connect',
  };

  arrayNullOverrides = ['Tags'];
}
