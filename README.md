# urineless

IRC chat client with optional end-to-end encryption. Use at your own risk.

## Stickers

Open the sticker picker with the **Sticker** button or `/sticker`. Local image
stickers can be placed in `<user-config>/urineless/stickers` (PNG, JPEG, or GIF).
The picker also includes Nekos and Tenor search. Nekos loads up to 20 stickers
per category, and **Mehr Sticker** appends results from further categories up
to 160. Tenor search requests up to 50 results.

Tenor search first scrapes direct GIF links from Tenor's public search page, so
it works without an API key when Tenor exposes media URLs in its HTML. This is
best-effort and can break if Tenor changes its page format. If `TENOR_API_KEY`
is set, the client uses Tenor's official API instead. The picker sends direct
HTTPS GIF URLs as normal chat messages, so other IRC clients can open them and
urineless clients can show them inline. Direct Tenor media links are also
recognized when pasted into chat.
