export function initialize() {
  const application = arguments[1] || arguments[0];

  // Provides the app config to all templates
  application.inject('controller', 'config', 'service:config');
  application.inject('component', 'config', 'service:config');

  const jQuery = window.jQuery;

  jQuery.__ajax = jQuery.ajax;
  jQuery.ajax = function() {
    // eslint-disable-next-line
    console.log('jQuery.ajax called:', ...arguments);
    return jQuery.__ajax(...arguments);
  };
}

export default {
  name: 'app-config',
  initialize,
};
