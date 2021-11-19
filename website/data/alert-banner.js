export const ALERT_BANNER_ACTIVE = true
// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  tag: 'ANNOUNCEMENT',
  url: 'https://www.hashicorp.com/blog/announcing-hashicorp-nomad-1-2',
  text:
    'Nomad 1.2.0 is now generally available, which includes 3 new major features and many improvements.',
  linkText: 'Learn more',
  // Set the expirationDate prop with a datetime string (e.g. '2020-01-31T12:00:00-07:00')
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: '2021-11-23T00:00:00-07:00',
}
