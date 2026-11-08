SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

UPDATE settings
SET value = 'false'
WHERE key IN (
    'turnstile_enabled',
    'tencent_captcha_enabled',
    'aliyun_captcha_enabled',
    'dingtalk_connect_enabled',
    'oidc_connect_enabled',
    'balance_low_notify_enabled',
    'channel_monitor_enabled',
    'channel_monitor_user_visible',
    'available_channels_enabled',
    'payment_enabled'
);
