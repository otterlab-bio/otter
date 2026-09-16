# fastq downsample

beaver_downsample <- function(R1,R2,
                              newR1,newR2,
                              ratio = 0.2,
                              use_conda_env = NULL){
  
  if (!is.null(use_conda_env)){
    use_conda_env <- paste("conda run -n",use_conda_env)
  }else{
    use_conda_env <- ""
  }
  if (file.exists(R1) & file.exists(R2)){
    R1_cmd <- glue::glue("{use_conda_env} seqtk sample -s100 {R1} {ratio} | gzip > {newR1} ")
    R2_cmd <- glue::glue("{use_conda_env} seqtk sample -s100 {R2} {ratio} | gzip > {newR2} ")
  }else{
    stop("the filepath of R1 and R2 is wrong")
  }
  fs::dir_create(dirname(newR1))
  fs::dir_create(dirname(newR2))
  if (is.numeric(ratio)){
    command <- glue::glue("{R1_cmd} && {R2_cmd}")
    system(command = command)
  }else{
    stop("the ratio should be numeric.")
  }
  return(list(
    newR1,newR2
  ))
}