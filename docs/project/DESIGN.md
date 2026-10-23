# PhotoKeep design system

## Direction

A photo-first desktop library inspired by the supplied Pinterest-style reference. A warm off-white outer canvas frames a white workspace, clear left navigation, one wide search-and-action bar, and a dense masonry gallery that puts image browsing first. The reference's social content is not copied: PhotoKeep shows actual local files and their real metadata.

## Color

- Canvas: warm off-white (`#f7f7f5`) with quiet neutral borders.
- Content surfaces: white and very pale gray (`#fbfbf9`, `#eeefec`).
- Accent: charcoal (`#17181a`) for the primary backup action and active navigation; keep status colors reserved for real states.
- Destructive actions and errors: muted red, reserved for those states.
- Keep text and interactive controls at readable contrast.

## Type and shape

Use the platform system sans stack and concise Thai labels. Keep hierarchy obvious through spacing and weight. Use square corners throughout, including controls, cards, thumbnails, and dialogs. Avoid glass blur, oversized hero cards, gradients, and decorative UI.

## Layout and behavior

- Use a labeled left navigation for the photo library, backup action, and settings.
- Put search, tag filters, refresh, and the primary backup action in the first viewport.
- Show images in a responsive masonry layout using their real aspect ratios and filename/date metadata.
- Keep AI image-analysis selection and progress visible without displacing the image grid.
- Center settings content in the available window and preserve folder, API key, and database actions.
- Preserve keyboard focus, reduced-motion behavior, responsive layouts, and existing workflows.
