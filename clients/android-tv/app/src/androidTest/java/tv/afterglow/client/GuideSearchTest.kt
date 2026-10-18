package tv.afterglow.client
import androidx.compose.runtime.*
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.semantics.SemanticsActions
import org.junit.Rule
import org.junit.Test
class GuideSearchTest {
 @get:Rule val compose=createComposeRule()
 @Test fun searchRequiresActivationAndRestoresButtonFocus(){
  compose.setContent{var query by remember{mutableStateOf("")};GuideSearch(query){query=it}}
  compose.onAllNodes(hasSetTextAction()).assertCountEquals(0)
  compose.onNodeWithText("⌕ Search").performSemanticsAction(SemanticsActions.RequestFocus)
  compose.onAllNodes(hasSetTextAction()).assertCountEquals(0)
  compose.onNodeWithText("⌕ Search").performSemanticsAction(SemanticsActions.OnClick)
  compose.onNode(hasSetTextAction()).performTextInput("Animation")
  compose.onNodeWithText("Apply search").performSemanticsAction(SemanticsActions.OnClick)
  compose.onAllNodes(hasSetTextAction()).assertCountEquals(0)
  compose.onNodeWithText("⌕ Search").assertIsFocused()
  compose.onNodeWithText("Animation").assertExists()
  compose.onNodeWithText("Clear search").performSemanticsAction(SemanticsActions.OnClick)
  compose.onNodeWithText("Animation").assertDoesNotExist()
 }
}
