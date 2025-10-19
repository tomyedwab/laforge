# Fix Parent Task Selection - Autocomplete Implementation

## Problem Analysis
The current TaskForm component uses a simple HTML `<select>` dropdown for parent task selection, which becomes unusable when there are many tasks. The dropdown loads all available tasks but presents them in a basic select element that:
- Shows only a few options at once
- Has no search/filter capability  
- Becomes unwieldy with large task lists
- Poor user experience for finding specific tasks

## Solution: Autocomplete Component
Replace the simple dropdown with an autocomplete component that provides:
- Search-as-you-type functionality
- Debounced API calls to avoid excessive requests
- Keyboard navigation support
- Clear selection capability
- Loading and error states
- Mobile-friendly interface

## Implementation Plan

### 1. Create Autocomplete Component
Create a reusable `Autocomplete` component in `/web-ui/src/components/Autocomplete.tsx` with:
- Search input with debouncing
- Dropdown results list
- Keyboard navigation (arrow keys, enter, escape)
- Loading spinner during API calls
- Error handling display
- Click outside to close functionality

### 2. Update TaskForm Component
Replace the current `<select>` elements for parent task and dependency selection with:
- Autocomplete components for both fields
- Maintain existing form validation
- Preserve current data loading logic
- Update form data handling for autocomplete values

### 3. API Integration
Enhance the API service to support search functionality:
- Add search parameter to `getTasks` method
- Implement debounced search calls
- Handle search result caching

### 4. Styling and UX
- Match existing form styling
- Add smooth animations for dropdown
- Ensure mobile responsiveness
- Add clear button for selection reset

## Technical Details

### Autocomplete Component Props
```typescript
interface AutocompleteProps {
  value: number | null;
  onChange: (value: number | null) => void;
  placeholder: string;
  disabled?: boolean;
  loadOptions: (search: string) => Promise<Array<{id: number, label: string}>>;
}
```

### Search Implementation
- Debounce search input at 300ms
- Minimum 2 characters before searching
- Load maximum 20 results per search
- Show "No results found" message when appropriate

### Error Handling
- Display API errors gracefully
- Allow retry on failed searches
- Fallback to "No results" state

## Testing Requirements
- Unit tests for Autocomplete component
- Integration tests for TaskForm with autocomplete
- Test keyboard navigation
- Test mobile touch interactions
- Test error scenarios

## Acceptance Criteria
- [ ] Users can type to search for parent tasks
- [ ] Search results update as user types (debounced)
- [ ] Users can select tasks from search results
- [ ] Selected task displays properly in the form
- [ ] Users can clear their selection
- [ ] Component works on mobile devices
- [ ] Keyboard navigation works properly
- [ ] Loading states are displayed during API calls
- [ ] Error messages are shown for failed searches
- [ ] Existing form validation still works
- [ ] Both parent task and dependency fields use autocomplete