# Color Palette Implementation Plan

## Current Analysis

The web UI currently uses a blue/gray color scheme with these main colors:
- Header background: `#2c3e50` (dark blue-gray)
- Primary buttons: `#3498db` (blue)
- Success states: `#27ae60` (green)
- Error states: `#e74c3c` (red)
- Background: `#f5f5f5` (light gray)

## New Color Palette

The specified palette is:
- **Jasmine**: `#fee787` (RGB: 254, 231, 135) - Light yellow
- **Brown**: `#a25b23` (RGB: 162, 91, 35) - Medium brown
- **Bistre**: `#2a200b` (RGB: 42, 32, 11) - Very dark brown
- **Lion**: `#bd9d68` (RGB: 189, 157, 104) - Light brown/tan
- **Black**: `#000000` (RGB: 0, 0, 0) - Pure black

## Implementation Strategy

### 1. CSS Custom Properties
Create a new color system using CSS custom properties in `src/index.css`:

```css
:root {
  /* Primary colors */
  --color-jasmine: #fee787;
  --color-brown: #a25b23;
  --color-bistre: #2a200b;
  --color-lion: #bd9d68;
  --color-black: #000000;
  
  /* Semantic colors */
  --color-primary: var(--color-brown);
  --color-primary-light: var(--color-lion);
  --color-primary-dark: var(--color-bistre);
  --color-accent: var(--color-jasmine);
  --color-background: #fafafa;
  --color-surface: #ffffff;
  --color-text: var(--color-bistre);
  --color-text-light: var(--color-lion);
  --color-border: var(--color-lion);
  
  /* Status colors */
  --color-success: var(--color-lion);
  --color-warning: var(--color-jasmine);
  --color-error: var(--color-brown);
  --color-info: var(--color-lion);
}
```

### 2. Component Updates

#### Header (`src/app.css` lines 42-55)
- Change `.app-header` background from `#2c3e50` to `var(--color-bistre)`
- Update navigation buttons to use new palette
- Update logout button from `#e74c3c` to `var(--color-brown)`

#### Task Status Colors (`src/components/TaskCard.tsx` lines 13-29)
Update the status and type color mappings:
```typescript
const statusColors = {
  todo: 'var(--color-lion)',
  'in-progress': 'var(--color-jasmine)',
  'in-review': 'var(--color-brown)',
  completed: 'var(--color-bistre)',
};

const typeColors = {
  EPIC: 'var(--color-bistre)',
  FEAT: 'var(--color-brown)',
  BUG: 'var(--color-brown)',
  PLAN: 'var(--color-lion)',
  DOC: 'var(--color-bistre)',
  ARCH: 'var(--color-lion)',
  DESIGN: 'var(--color-jasmine)',
  TEST: 'var(--color-bistre)',
};
```

#### Buttons and Forms
- Update primary buttons from `#3498db` to `var(--color-brown)`
- Update hover states to use `var(--color-lion)`
- Update form focus states to use new palette

#### Cards and Surfaces
- Update card backgrounds to work with new color scheme
- Ensure proper contrast ratios for accessibility

### 3. Specific CSS Updates

#### Background and Text
- Body background: Change from `#f5f5f5` and `#333` to new palette
- Ensure sufficient contrast ratios (WCAG 2.1 AA compliance)

#### Status Indicators
- Connection status colors
- Review status badges
- Step status indicators

#### Interactive Elements
- Button hover/focus states
- Tab active states
- Form input focus states

### 4. Dark Mode Considerations
The current CSS includes dark mode support that needs to be updated:
- Update dark mode color variables
- Ensure proper contrast in both light and dark modes

### 5. Testing Checklist
- [ ] All UI elements use new color palette
- [ ] Sufficient contrast ratios (4.5:1 for normal text, 3:1 for large text)
- [ ] Consistent color usage across components
- [ ] Dark mode compatibility
- [ ] Mobile responsiveness maintained
- [ ] No hardcoded colors remaining

## Implementation Steps

1. **Create CSS variables** in `src/index.css`
2. **Update main app.css** to use CSS variables
3. **Update component-specific colors** in React components
4. **Test contrast ratios** and accessibility
5. **Verify dark mode** compatibility
6. **Test mobile** responsiveness
7. **Final review** and cleanup

## Files to Modify

1. `src/index.css` - Add CSS custom properties
2. `src/app.css` - Update main styles
3. `src/components/TaskCard.tsx` - Update status/type colors
4. `src/components/ProjectSelector.css` - Update dropdown colors
5. Other component files as needed for specific color references