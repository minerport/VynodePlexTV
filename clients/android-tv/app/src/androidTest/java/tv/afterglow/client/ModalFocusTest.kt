package tv.afterglow.client

import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.tv.material3.*
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalTestApi::class)
class ModalFocusTest {
 @get:Rule val compose=createComposeRule()
 @Test fun programModalOwnsRemoteFocus(){
  compose.setContent{MaterialTheme{var open by remember{mutableStateOf(false)}
   Button(onClick={open=true}){Text("Guide program")}
   if(open)DetailsModal(onClose={open=false}){focus->Column{
    Button(onClick={open=false},modifier=Modifier.focusRequester(focus)){Text("Back")}
    Button(onClick={}){Text("Watch Live")}
   }}
  }}
  compose.onNodeWithText("Guide program").performSemanticsAction(SemanticsActions.OnClick)
  compose.waitUntil(5000){compose.onAllNodesWithText("Back").fetchSemanticsNodes().isNotEmpty()}
  compose.onNodeWithText("Back").assertIsFocused()
  compose.onNodeWithText("Back").performKeyInput{pressKey(Key.DirectionDown)}
  compose.onNodeWithText("Watch Live").assertIsFocused()
  compose.onNodeWithText("Guide program").assertIsNotFocused()
 }
}
