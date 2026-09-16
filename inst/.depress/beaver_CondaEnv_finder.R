beaver_CondaEnv_finder <- function(){
  
  checkSystemResult <- function(result) {
    if (is.character(result) && is.null(attr(result, "status"))) {
      sys_output <- result
      return(sys_output)
    }
    
    if (is.numeric(result)) {
      status <- result
    } else {
      status <- if (!is.null(attr(result, "status"))) attr(result, "status") else 0
    }
    
    if (status != 0) message("--Command failed with status: ", status)
    
    return(status)
  }
  
  envs <- system(
    command = "conda env list",
    intern = F, wait = TRUE, ignore.stdout = FALSE, ignore.stderr = FALSE
  ) 
  if (checkSystemResult(envs) == 0){
    envs <- system(
      command = "conda env list",
      intern = T
    )  %>% 
      {
      temp <- .
      temp <- temp[temp != ""]
      temp <- temp[!grepl("#",temp)]
      temp
    }
    for (i in 1:length(envs)){
      envs[i] <- stringr::str_split(envs[i],"\\s+")[1][[1]] %>% 
        stringr::str_trim()
    }
  } else {
    envs <- NULL
  }
  
  return(envs)
  
}

